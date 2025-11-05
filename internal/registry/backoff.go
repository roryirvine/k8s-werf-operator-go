// Package registry provides OCI registry interactions for pulling bundle information.
package registry

import (
	"math/rand"
	"time"
)

// CalculateBackoff returns the backoff duration for the given number of consecutive failures.
// Uses exponential backoff: base 30s * 2^failures, capped at 8 minutes (480s).
// Examples:
//   - 0 failures: 30s
//   - 1 failure: 1m (30s * 2^1)
//   - 2 failures: 2m (30s * 2^2)
//   - 3 failures: 4m (30s * 2^3)
//   - 4+ failures: 8m (capped)
func CalculateBackoff(consecutiveFailures int32) time.Duration {
	const baseDuration = 30 * time.Second
	const maxDuration = 8 * time.Minute

	if consecutiveFailures <= 0 {
		return baseDuration
	}

	// Calculate 2^failures (bit shift for power of 2)
	multiplier := 1 << uint(consecutiveFailures)
	duration := baseDuration * time.Duration(multiplier)

	if duration > maxDuration {
		return maxDuration
	}
	return duration
}

// AddJitter returns the input duration with 0-20% random jitter added.
// Prevents thundering herd when multiple bundles have same poll interval.
// Examples with 15m interval:
//   - without jitter: all bundles poll at :00 minutes
//   - with jitter: polls spread over 12m-18m range
func AddJitter(interval time.Duration) time.Duration {
	if interval <= 0 {
		return interval
	}

	// Calculate 20% of the interval
	jitterAmount := interval / 5

	// Random 0 to jitterAmount
	jitter := time.Duration(rand.Intn(int(jitterAmount)))

	return interval + jitter
}
