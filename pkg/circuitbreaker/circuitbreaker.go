package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// CircuitState is the current circuit breaker state.
type CircuitState int

const (
	// Closed allows requests to pass through
	Closed CircuitState = iota
	// Open blocks all requests
	Open
	// HalfOpen allows limited requests to test recovery
	HalfOpen
)

// CircuitBreaker guards calls and opens the circuit after repeated failures.
type CircuitBreaker interface {
	Call(func() error) error
	State() CircuitState
	Reset()
}

type Config struct {
	FailureThreshold int           // Number of failures before opening
	RecoveryTimeout  time.Duration // Time to wait before trying HalfOpen
	SuccessThreshold int           // Number of successes needed to close from HalfOpen
}

func DefaultConfig() *Config {
	return &Config{
		FailureThreshold: 5,
		RecoveryTimeout:  60 * time.Second,
		SuccessThreshold: 3,
	}
}

type circuitBreaker struct {
	config      *Config
	state       CircuitState
	failures    int
	successes   int
	lastFailure time.Time
	nextAttempt time.Time
	mutex       sync.RWMutex
}

// NewCircuitBreaker returns a circuit breaker and applies defaults when config is nil.
func NewCircuitBreaker(config *Config) CircuitBreaker {
	if config == nil {
		config = DefaultConfig()
	}

	return &circuitBreaker{
		config: config,
		state:  Closed,
	}
}

func (cb *circuitBreaker) shouldAllowRequest() bool {
	// Transition Open -> HalfOpen after the recovery timeout.
	if cb.state == Open && time.Now().After(cb.nextAttempt) {
		cb.state = HalfOpen
		cb.successes = 0
	}
	return cb.state != Open
}

func (cb *circuitBreaker) Call(fn func() error) error {
	cb.mutex.Lock()
	shouldAllow := cb.shouldAllowRequest()
	cb.mutex.Unlock()

	if !shouldAllow {
		return ErrCircuitOpen
	}

	// Never call user code while holding locks.
	err := fn()
	cb.mutex.Lock()
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
	cb.mutex.Unlock()
	return err
}

func (cb *circuitBreaker) State() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

func (cb *circuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.state = Closed
	cb.failures = 0
	cb.successes = 0
}

func (cb *circuitBreaker) recordFailure() {
	cb.failures++
	cb.lastFailure = time.Now()

	switch cb.state {
	case Closed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.state = Open
			cb.nextAttempt = time.Now().Add(cb.config.RecoveryTimeout)
		}
	case HalfOpen:
		cb.state = Open
		cb.nextAttempt = time.Now().Add(cb.config.RecoveryTimeout)
	}
}

func (cb *circuitBreaker) recordSuccess() {
	cb.failures = 0

	if cb.state == HalfOpen {
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.state = Closed
			cb.successes = 0
		}
	}
}

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreakerMetrics exposes current state and counters.
type CircuitBreakerMetrics struct {
	State        CircuitState
	FailureCount int
	SuccessCount int
	LastFailure  time.Time
	NextAttempt  time.Time
}

func (cb *circuitBreaker) GetMetrics() CircuitBreakerMetrics {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return CircuitBreakerMetrics{
		State:        cb.state,
		FailureCount: cb.failures,
		SuccessCount: cb.successes,
		LastFailure:  cb.lastFailure,
		NextAttempt:  cb.nextAttempt,
	}
}
