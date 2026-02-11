package constants

import "time"

// RFC 3339 date-time format string.
// Use this format for all date-time serialization and communication with external systems.
const RFC3339DateTimeFormat = "2006-01-02T15:04:05Z07:00"

// Default rate limiting configuration
const (
	// DefaultRateLimitRequests is the default number of requests allowed per time window
	DefaultRateLimitRequests = 100
	// DefaultRateLimitWindow is the default time window for rate limiting
	DefaultRateLimitWindowMinutes = 1
)

// DefaultRateLimitWindow returns the default rate limit window duration
func DefaultRateLimitWindow() time.Duration {
	return time.Duration(DefaultRateLimitWindowMinutes) * time.Minute
}
