package ratelimit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/time/rate"
)

type Logger interface {
	Error(msg string, args ...interface{})
}

func generateUniqueID() string {
	bytes := make([]byte, 8)

	rand.Read(bytes)

	return hex.EncodeToString(bytes)
}

// RateLimiter defines the strategy interface for rate limiting
type RateLimiter interface {
	GetLimitDetails() (int, time.Duration)
	IsLimited(key string) (bool, error)
	Close() error
}

// InMemoryRateLimiter implements token bucket rate limiting for single instances
type InMemoryRateLimiter struct {
	requests int
	window   time.Duration

	mu       sync.Mutex
	limiters map[string]*keyedLimiter
	ops      uint64
}

type keyedLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewInMemoryRateLimiter(requests int, window time.Duration) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		requests: requests,
		window:   window,
		limiters: make(map[string]*keyedLimiter),
	}
}

func (r *InMemoryRateLimiter) IsLimited(key string) (bool, error) {
	if key == "" {
		key = "__empty__"
	}

	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	k, ok := r.limiters[key]
	if !ok {
		rps := float64(r.requests) / r.window.Seconds()
		k = &keyedLimiter{
			limiter:  rate.NewLimiter(rate.Limit(rps), r.requests),
			lastSeen: now,
		}
		r.limiters[key] = k
	} else {
		k.lastSeen = now
	}

	// Opportunistic cleanup to avoid unbounded growth.
	// We run it infrequently and only remove keys that have not been seen for
	// a while. This is a pragmatic tradeoff for a starter template.
	r.ops++
	if r.ops%1024 == 0 {
		cutoff := now.Add(-2 * r.window)
		for kKey, kVal := range r.limiters {
			if kVal.lastSeen.Before(cutoff) {
				delete(r.limiters, kKey)
			}
		}
	}

	return !k.limiter.Allow(), nil
}

func (r *InMemoryRateLimiter) Close() error {
	return nil
}

// RedisRateLimiter implements sliding window rate limiting for distributed systems
type RedisRateLimiter struct {
	client    *redis.Client
	requests  int
	window    time.Duration
	keyPrefix string
	logger    Logger
}

func NewRedisRateLimiter(client *redis.Client, requests int, window time.Duration, logger Logger) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:    client,
		requests:  requests,
		window:    window,
		keyPrefix: "ratelimit:",
		logger:    logger,
	}
}

func (r *RedisRateLimiter) GetLimitDetails() (int, time.Duration) {
	return r.requests, r.window
}

func (r *InMemoryRateLimiter) GetLimitDetails() (int, time.Duration) {
	return r.requests, r.window
}

func (r *RedisRateLimiter) IsLimited(key string) (bool, error) {
	ctx := context.Background()
	fullKey := key
	if r.keyPrefix != "" && !strings.HasPrefix(key, r.keyPrefix) {
		fullKey = r.keyPrefix + key
	}
	now := time.Now().Unix()
	memberID := generateUniqueID()

	// Atomic sliding window rate limiting.
	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local expire = tonumber(ARGV[4])
		local memberId = ARGV[5]
		
		-- Remove old entries outside the window
		redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
		
		-- Count current requests in window
		local count = redis.call('ZCARD', key)
		
		-- Check if limit exceeded
		if count >= limit then
			return 1 -- Rate limited
		end
		
		-- Add current request with unique member ID
		redis.call('ZADD', key, now, memberId)
		
		-- Set expiration on the key for cleanup
		redis.call('EXPIRE', key, expire)
		
		return 0 -- Not rate limited
	`

	result, err := r.client.Eval(ctx, script, []string{fullKey}, now, int64(r.window.Seconds()), r.requests, int64((r.window * 2).Seconds()), memberID).Result()
	if err != nil {
		if r.logger != nil {
			r.logger.Error("Redis rate limit script execution failed", "key", fullKey, "error", err)
		}
		// Return error instead of silently allowing: limiting is a security control.
		return false, fmt.Errorf("rate limiter Redis error: %w", err)
	}
	return result.(int64) == 1, nil
}

// The Redis client is owned by the ApplicationConfig and closed there
func (r *RedisRateLimiter) Close() error {
	return nil
}

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	Requests int
	Window   time.Duration
	Redis    *redis.Client // Optional, if nil uses in-memory
	Logger   Logger        // Optional logger for Redis operations
}

// NewRateLimiter creates a rate limiter based on configuration
func NewRateLimiter(config *RateLimitConfig) RateLimiter {
	if config.Redis != nil {
		return NewRedisRateLimiter(config.Redis, config.Requests, config.Window, config.Logger)
	}
	return NewInMemoryRateLimiter(config.Requests, config.Window)
}
