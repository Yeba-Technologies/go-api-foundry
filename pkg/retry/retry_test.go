package retry

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errTimeout    = errors.New("connection timeout")
	errBusiness   = errors.New("invalid input")
	errUnavail    = errors.New("service unavailable")
	errConnRefuse = errors.New("connection refused")
)

func fastConfig(maxAttempts int) *Config {
	return &Config{
		MaxAttempts: maxAttempts,
		BaseDelay:   time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
		Multiplier:  2.0,
	}
}

func TestExponentialBackoff_SucceedsFirstTry(t *testing.T) {
	eb := NewExponentialBackoff(fastConfig(3))

	err := eb.Execute(func() error { return nil })
	assert.NoError(t, err)
}

func TestExponentialBackoff_RetriesAndSucceeds(t *testing.T) {
	eb := NewExponentialBackoff(fastConfig(5))

	var calls atomic.Int32
	err := eb.Execute(func() error {
		n := calls.Add(1)
		if n < 3 {
			return errTimeout
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, int32(3), calls.Load())
}

func TestExponentialBackoff_ExhaustsRetries(t *testing.T) {
	eb := NewExponentialBackoff(fastConfig(3))

	err := eb.Execute(func() error { return errTimeout })

	require.Error(t, err)
	assert.True(t, IsMaxRetriesExceeded(err))

	var mre *MaxRetriesExceededError
	require.ErrorAs(t, err, &mre)
	assert.Equal(t, 3, mre.MaxAttempts)
	assert.ErrorIs(t, mre.Unwrap(), errTimeout)
}

func TestExponentialBackoff_NonRetryable_StopsImmediately(t *testing.T) {
	eb := NewExponentialBackoff(fastConfig(5))

	var calls atomic.Int32
	err := eb.Execute(func() error {
		calls.Add(1)
		return errBusiness
	})

	assert.ErrorIs(t, err, errBusiness)
	assert.Equal(t, int32(1), calls.Load())
}

func TestExponentialBackoff_NilConfig_UsesDefaults(t *testing.T) {
	eb := NewExponentialBackoff(nil)
	assert.NotNil(t, eb.config)
	assert.Equal(t, 3, eb.config.MaxAttempts)
}

func TestExponentialBackoff_CalculateDelay_Bounds(t *testing.T) {
	eb := NewExponentialBackoff(&Config{
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Multiplier: 2.0,
	})

	// attempt 1: base = 10ms, jitter adds 0-50% → 10ms–15ms
	d := eb.calculateDelay(1)
	assert.GreaterOrEqual(t, d, 10*time.Millisecond)
	assert.LessOrEqual(t, d, 15*time.Millisecond)

	// attempt 3: base = 10*4 = 40ms, jitter → 40ms–60ms
	d = eb.calculateDelay(3)
	assert.GreaterOrEqual(t, d, 40*time.Millisecond)
	assert.LessOrEqual(t, d, 60*time.Millisecond)
}

func TestExponentialBackoff_CalculateDelay_CappedAtMax(t *testing.T) {
	eb := NewExponentialBackoff(&Config{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   200 * time.Millisecond,
		Multiplier: 10.0,
	})

	// attempt 3 would be 100*100 = 10s, but capped to 200ms + jitter → 200ms–300ms
	d := eb.calculateDelay(3)
	assert.LessOrEqual(t, d, 300*time.Millisecond)
}

func TestFixedDelay_SucceedsFirstTry(t *testing.T) {
	fd := NewFixedDelay(fastConfig(3))

	err := fd.Execute(func() error { return nil })
	assert.NoError(t, err)
}

func TestFixedDelay_RetriesAndSucceeds(t *testing.T) {
	fd := NewFixedDelay(fastConfig(3))

	var calls atomic.Int32
	err := fd.Execute(func() error {
		n := calls.Add(1)
		if n < 2 {
			return errConnRefuse
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load())
}

func TestFixedDelay_ExhaustsRetries(t *testing.T) {
	fd := NewFixedDelay(fastConfig(2))

	err := fd.Execute(func() error { return errUnavail })

	require.Error(t, err)
	assert.True(t, IsMaxRetriesExceeded(err))
}

func TestFixedDelay_NonRetryable_StopsImmediately(t *testing.T) {
	fd := NewFixedDelay(fastConfig(5))

	var calls atomic.Int32
	err := fd.Execute(func() error {
		calls.Add(1)
		return errBusiness
	})

	assert.ErrorIs(t, err, errBusiness)
	assert.Equal(t, int32(1), calls.Load())
}

func TestFixedDelay_NilConfig_UsesDefaults(t *testing.T) {
	fd := NewFixedDelay(nil)
	assert.NotNil(t, fd.config)
	assert.Equal(t, 3, fd.config.MaxAttempts)
}

func TestIsRetryable_Patterns(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"timeout", errors.New("request timeout"), true},
		{"temporary failure", errors.New("temporary failure in name resolution"), true},
		{"service unavailable", errors.New("service unavailable"), true},
		{"too many requests", errors.New("too many requests"), true},
		{"mixed case", errors.New("Connection Refused"), true},
		{"unrelated error", errors.New("not found"), false},
		{"business error", errors.New("invalid email"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isRetryable(tt.err))
		})
	}
}

func TestMaxRetriesExceededError_Message(t *testing.T) {
	e := &MaxRetriesExceededError{LastError: errTimeout, MaxAttempts: 3}
	assert.Equal(t, "max retries exceeded", e.Error())
}

func TestMaxRetriesExceededError_Unwrap(t *testing.T) {
	e := &MaxRetriesExceededError{LastError: errTimeout, MaxAttempts: 3}
	assert.ErrorIs(t, e, errTimeout)
}

func TestIsMaxRetriesExceeded_True(t *testing.T) {
	e := &MaxRetriesExceededError{LastError: errTimeout, MaxAttempts: 3}
	assert.True(t, IsMaxRetriesExceeded(e))
}

func TestIsMaxRetriesExceeded_False(t *testing.T) {
	assert.False(t, IsMaxRetriesExceeded(errBusiness))
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, 3, cfg.MaxAttempts)
	assert.Equal(t, 100*time.Millisecond, cfg.BaseDelay)
	assert.Equal(t, 30*time.Second, cfg.MaxDelay)
	assert.Equal(t, 2.0, cfg.Multiplier)
}
