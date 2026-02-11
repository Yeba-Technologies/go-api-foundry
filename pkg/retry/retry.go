package retry

import (
	"errors"
	"math"
	"strings"
	"time"
)

type RetryPolicy interface {
	Execute(func() error) error
}

type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultConfig returns conservative defaults for backoff retries.
func DefaultConfig() *Config {
	return &Config{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
	}
}

// ExponentialBackoff retries with exponential delay between attempts.
type ExponentialBackoff struct {
	config *Config
}

// NewExponentialBackoff applies defaults when config is nil.
func NewExponentialBackoff(config *Config) *ExponentialBackoff {
	if config == nil {
		config = DefaultConfig()
	}
	return &ExponentialBackoff{config: config}
}

func (eb *ExponentialBackoff) Execute(fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= eb.config.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt == eb.config.MaxAttempts {
			break
		}

		if !isRetryable(err) {
			return err
		}

		delay := eb.calculateDelay(attempt)
		time.Sleep(delay)
	}

	return &MaxRetriesExceededError{
		LastError:   lastErr,
		MaxAttempts: eb.config.MaxAttempts,
	}
}

func (eb *ExponentialBackoff) calculateDelay(attempt int) time.Duration {
	delay := float64(eb.config.BaseDelay) * math.Pow(eb.config.Multiplier, float64(attempt-1))
	if delay > float64(eb.config.MaxDelay) {
		delay = float64(eb.config.MaxDelay)
	}

	return time.Duration(delay)
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"service unavailable",
		"too many requests",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// FixedDelay retries with a constant delay between attempts.
type FixedDelay struct {
	config *Config
}

// NewFixedDelay applies defaults when config is nil.
func NewFixedDelay(config *Config) *FixedDelay {
	if config == nil {
		config = DefaultConfig()
	}
	return &FixedDelay{config: config}
}

func (fd *FixedDelay) Execute(fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= fd.config.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt == fd.config.MaxAttempts {
			break
		}

		if !isRetryable(err) {
			return err
		}
		time.Sleep(fd.config.BaseDelay)
	}

	return &MaxRetriesExceededError{
		LastError:   lastErr,
		MaxAttempts: fd.config.MaxAttempts,
	}
}

// MaxRetriesExceededError indicates that all retry attempts were exhausted.
type MaxRetriesExceededError struct {
	LastError   error
	MaxAttempts int
}

func (e *MaxRetriesExceededError) Error() string {
	return "max retries exceeded"
}

func (e *MaxRetriesExceededError) Unwrap() error {
	return e.LastError
}

// IsMaxRetriesExceeded reports whether err is a MaxRetriesExceededError.
func IsMaxRetriesExceeded(err error) bool {
	var maxRetriesErr *MaxRetriesExceededError
	return errors.As(err, &maxRetriesErr)
}
