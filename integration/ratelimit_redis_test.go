//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/integration/testhelpers"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/ratelimit"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/suite"
)

// RateLimitRedisTestSuite tests the Redis-backed sliding-window rate limiter
// against a real Redis instance.
type RateLimitRedisTestSuite struct {
	suite.Suite
	ctx     context.Context
	client  *redis.Client
	limiter ratelimit.RateLimiter
}

func (suite *RateLimitRedisTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	redisC := testhelpers.StartRedis(suite.ctx, suite.T())

	suite.client = redis.NewClient(&redis.Options{
		Addr: redisC.Addr(),
	})

	err := suite.client.Ping(suite.ctx).Err()
	suite.Require().NoError(err, "should ping test redis")

	// Allow 3 requests per 1-second window for testing.
	suite.limiter = ratelimit.NewRedisRateLimiter(suite.client, 3, time.Second, nil)
}

func (suite *RateLimitRedisTestSuite) TearDownSuite() {
	if suite.client != nil {
		suite.client.Close()
	}
}

func (suite *RateLimitRedisTestSuite) SetupTest() {
	// Flush all keys so each test starts clean.
	err := suite.client.FlushAll(suite.ctx).Err()
	suite.Require().NoError(err, "FlushAll must succeed for a clean test state")
}

// TestUnderLimit verifies requests within the allowed window are not limited.
func (suite *RateLimitRedisTestSuite) TestUnderLimit() {
	for i := 0; i < 3; i++ {
		limited, err := suite.limiter.IsLimited("client-x")
		suite.Require().NoError(err)
		suite.False(limited, "request %d should not be limited", i+1)
	}
}

// TestOverLimit verifies the 4th request in a 3-per-second window is blocked.
func (suite *RateLimitRedisTestSuite) TestOverLimit() {
	for i := 0; i < 3; i++ {
		_, err := suite.limiter.IsLimited("client-y")
		suite.Require().NoError(err)
	}

	limited, err := suite.limiter.IsLimited("client-y")
	suite.Require().NoError(err)
	suite.True(limited, "4th request should be rate limited")
}

// TestPerKeyIsolation ensures rate limits are per-key.
func (suite *RateLimitRedisTestSuite) TestPerKeyIsolation() {
	// Exhaust key-a
	for i := 0; i < 3; i++ {
		_, err := suite.limiter.IsLimited("key-a")
		suite.Require().NoError(err)
	}
	limitedA, err := suite.limiter.IsLimited("key-a")
	suite.Require().NoError(err)
	suite.True(limitedA)

	// key-b should still have quota
	limitedB, err := suite.limiter.IsLimited("key-b")
	suite.Require().NoError(err)
	suite.False(limitedB, "key-b should not be affected by key-a exhaustion")
}

// TestWindowExpiry verifies the limiter resets after the window elapses.
func (suite *RateLimitRedisTestSuite) TestWindowExpiry() {
	for i := 0; i < 3; i++ {
		_, err := suite.limiter.IsLimited("key-expiry")
		suite.Require().NoError(err)
	}
	limited, err := suite.limiter.IsLimited("key-expiry")
	suite.Require().NoError(err)
	suite.True(limited)

	// Wait for the window to expire
	time.Sleep(1100 * time.Millisecond)

	limited, err := suite.limiter.IsLimited("key-expiry")
	suite.Require().NoError(err)
	suite.False(limited, "should be allowed after window expires")
}

func TestRateLimitRedisSuite(t *testing.T) {
	suite.Run(t, new(RateLimitRedisTestSuite))
}
