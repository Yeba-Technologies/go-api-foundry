# Design Patterns

This section summarizes the main design patterns used in this template and where they live.

## Implemented Patterns

### 1. Strategy Pattern - Rate Limiting
**Location:** `pkg/ratelimit/strategy.go`

The Strategy Pattern eliminates conditional logic for different rate limiting implementations (in-memory vs Redis-based). The `RateLimiter` interface allows seamless switching between implementations without changing client code.

**Notes:**
- Clean separation of rate limiting algorithms
- Easy to add new rate limiting strategies
- Testable implementations
- Configurable at runtime

Implementation details:

- In-memory limiter is keyed per client (token bucket per key)
- Redis limiter uses a sliding-window approach with an atomic Lua script

**Usage:**
```go
// Create rate limiter using strategy pattern
config := &ratelimit.RateLimitConfig{
    Requests: 100,
    Window:   time.Minute,
    Redis:    redisClient, // optional
}
rateLimiter := ratelimit.NewRateLimiter(config)
```

### 2. Factory Pattern - Domain-Specific Service Creation
**Location:** `domain/*/factory.go` (e.g., `domain/waitlist/factory.go`, `domain/monitoring/factory.go`)

The Factory Pattern is implemented per domain to create services and controllers, promoting Domain-Driven Design (DDD) principles by keeping component instantiation within bounded contexts.

**Benefits:**
- Decentralized dependency management per domain
- Adheres to Single Responsibility Principle (SRP)
- Prevents God object anti-pattern in global factories
- Easy mocking for unit tests within domains
- Loose coupling and high cohesion

**Usage:**
```go
// Domain-specific factory instantiation
waitlistFactory := waitlist.NewWaitlistServiceFactory(db, logger)
controller := waitlistFactory.CreateController()

// Or for monitoring
monitoringFactory := monitoring.NewMonitoringControllerFactory(db, logger, redisClient)
controller := monitoringFactory.CreateController()
```

### 3. Circuit Breaker Pattern - Fault Tolerance
**Location:** `pkg/circuitbreaker/circuitbreaker.go`

The Circuit Breaker Pattern provides resilience against cascading failures by monitoring service health and preventing calls to failing services.

**Benefits:**
- Prevents system overload during failures
- Fast failure detection and recovery
- Automatic recovery testing
- Configurable failure thresholds

**Usage:**
```go
cb := circuitbreaker.NewCircuitBreaker(circuitbreaker.Config{
    FailureThreshold: 5,
    RecoveryTimeout:  time.Minute,
    MonitoringPeriod: time.Minute,
})

result, err := cb.Execute(func() (interface{}, error) {
    return someExternalCall()
})
```

### 4. Retry Pattern - Transient Failure Handling
**Location:** `pkg/retry/retry.go`

The Retry Pattern handles transient failures with exponential backoff and configurable retry strategies.

**Benefits:**
- Automatic retry for transient failures
- Configurable backoff strategies
- Prevents thundering herd problems
- Comprehensive error handling

**Usage:**
```go
retryConfig := retry.Config{
    MaxAttempts: 3,
    BaseDelay:   time.Second,
    MaxDelay:    time.Minute,
}

result, err := retry.WithExponentialBackoff(retryConfig, func() error {
    return someUnreliableOperation()
})
```

## Integration Points

### Router Service
The router uses the Strategy Pattern for rate limiting:

- Environment-based configuration
- Automatic fallback to in-memory if Redis is not configured or unreachable

### Domain Layer
The domain layer uses domain-specific Factory Patterns for component creation:
- Decentralized controller and service instantiation per bounded context
- Proper dependency injection within domains
- Testable service creation aligned with DDD principles

## Configuration

All patterns support environment-based configuration:

```bash
# Rate Limiting
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=1m

# Redis (optional)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=yourpassword
```

## Testing

Interfaces make mocking straightforward:

```go
// Mock rate limiter for testing
type mockRateLimiter struct{}
func (m *mockRateLimiter) GetLimitDetails() (int, time.Duration) { return 0, 0 }
func (m *mockRateLimiter) IsLimited(key string) (bool, error) { return false, nil }
func (m *mockRateLimiter) Close() error { return nil }

// Use in tests
rateLimiter = &mockRateLimiter{}
```

## Summary

1. **Maintainability:** Clean interfaces and separation of concerns with DDD alignment
2. **Testability:** Dependency injection enables comprehensive testing per domain
3. **Scalability:** Strategy pattern allows horizontal scaling with Redis
4. **Resilience:** Circuit breaker and retry patterns handle failures gracefully
5. **Flexibility:** Domain-specific factories enable easy component swapping without global coupling
6. **Operational readiness:** Reusable building blocks for timeouts, retries, and limiting

## Future Enhancements

- Add configuration validation
- Create additional integration tests for failure modes (Redis down, timeouts, etc.)