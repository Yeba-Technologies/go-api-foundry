package ratelimit

import (
	"testing"
	"time"
)

func TestInMemoryRateLimiter_IsLimited_IsPerKey(t *testing.T) {
	limiter := NewInMemoryRateLimiter(1, time.Second)

	limited, err := limiter.IsLimited("client-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limited {
		t.Fatalf("first request for client-a should not be limited")
	}

	limited, err = limiter.IsLimited("client-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !limited {
		t.Fatalf("second immediate request for client-a should be limited")
	}

	limited, err = limiter.IsLimited("client-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limited {
		t.Fatalf("first request for client-b should not be limited (per-key limiter)")
	}
}
