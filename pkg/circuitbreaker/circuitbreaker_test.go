package circuitbreaker

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errSimulated = errors.New("simulated failure")

func TestNewCircuitBreaker_DefaultConfig(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	assert.Equal(t, Closed, cb.State())
}

func TestNewCircuitBreaker_CustomConfig(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 2, RecoveryTimeout: time.Second, SuccessThreshold: 1})
	assert.Equal(t, Closed, cb.State())
}

func TestCircuitBreaker_ClosedState_SuccessfulCalls(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 3, RecoveryTimeout: time.Second, SuccessThreshold: 1})

	for i := 0; i < 5; i++ {
		err := cb.Call(func() error { return nil })
		assert.NoError(t, err)
	}
	assert.Equal(t, Closed, cb.State())
}

func TestCircuitBreaker_OpensAfterFailureThreshold(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 3, RecoveryTimeout: time.Minute, SuccessThreshold: 1})

	for i := 0; i < 3; i++ {
		_ = cb.Call(func() error { return errSimulated })
	}
	assert.Equal(t, Open, cb.State())
}

func TestCircuitBreaker_OpenState_RejectsImmediately(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 1, RecoveryTimeout: time.Minute, SuccessThreshold: 1})

	_ = cb.Call(func() error { return errSimulated })
	assert.Equal(t, Open, cb.State())

	err := cb.Call(func() error { return nil })
	assert.ErrorIs(t, err, ErrCircuitOpen)
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 1, RecoveryTimeout: 10 * time.Millisecond, SuccessThreshold: 1})

	_ = cb.Call(func() error { return errSimulated })
	assert.Equal(t, Open, cb.State())

	time.Sleep(20 * time.Millisecond)

	err := cb.Call(func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, Closed, cb.State())
}

func TestCircuitBreaker_HalfOpen_FailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 1, RecoveryTimeout: 10 * time.Millisecond, SuccessThreshold: 2})

	_ = cb.Call(func() error { return errSimulated })
	assert.Equal(t, Open, cb.State())

	time.Sleep(20 * time.Millisecond)

	err := cb.Call(func() error { return errSimulated })
	assert.ErrorIs(t, err, errSimulated)
	assert.Equal(t, Open, cb.State())
}

func TestCircuitBreaker_HalfOpen_RequiresSuccessThreshold(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 1, RecoveryTimeout: 10 * time.Millisecond, SuccessThreshold: 3})

	_ = cb.Call(func() error { return errSimulated })
	time.Sleep(20 * time.Millisecond)

	_ = cb.Call(func() error { return nil })
	_ = cb.Call(func() error { return nil })
	_ = cb.Call(func() error { return nil })
	assert.Equal(t, Closed, cb.State())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 1, RecoveryTimeout: time.Minute, SuccessThreshold: 1})

	_ = cb.Call(func() error { return errSimulated })
	assert.Equal(t, Open, cb.State())

	cb.Reset()
	assert.Equal(t, Closed, cb.State())

	err := cb.Call(func() error { return nil })
	assert.NoError(t, err)
}

func TestCircuitBreaker_FailuresBelowThreshold_StaysClosed(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 5, RecoveryTimeout: time.Minute, SuccessThreshold: 1})

	for i := 0; i < 4; i++ {
		_ = cb.Call(func() error { return errSimulated })
	}
	assert.Equal(t, Closed, cb.State())
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 3, RecoveryTimeout: time.Minute, SuccessThreshold: 1})

	_ = cb.Call(func() error { return errSimulated })
	_ = cb.Call(func() error { return errSimulated })
	_ = cb.Call(func() error { return nil })

	_ = cb.Call(func() error { return errSimulated })
	_ = cb.Call(func() error { return errSimulated })
	assert.Equal(t, Closed, cb.State())
}

func TestCircuitBreaker_PropagatesUserError(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	userErr := errors.New("business logic error")

	err := cb.Call(func() error { return userErr })
	assert.ErrorIs(t, err, userErr)
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 100, RecoveryTimeout: time.Minute, SuccessThreshold: 1})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cb.Call(func() error { return nil })
			_ = cb.Call(func() error { return errSimulated })
		}()
	}
	wg.Wait()
	state := cb.State()
	assert.Contains(t, []CircuitState{Closed, Open}, state)
}

func TestCircuitBreaker_GetMetrics(t *testing.T) {
	cb := NewCircuitBreaker(&Config{FailureThreshold: 3, RecoveryTimeout: time.Minute, SuccessThreshold: 1})
	impl := cb.(*circuitBreaker)

	_ = cb.Call(func() error { return errSimulated })
	_ = cb.Call(func() error { return errSimulated })

	m := impl.GetMetrics()
	assert.Equal(t, Closed, m.State)
	assert.Equal(t, 2, m.FailureCount)
	assert.False(t, m.LastFailure.IsZero())
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, 5, cfg.FailureThreshold)
	assert.Equal(t, 60*time.Second, cfg.RecoveryTimeout)
	assert.Equal(t, 3, cfg.SuccessThreshold)
}
