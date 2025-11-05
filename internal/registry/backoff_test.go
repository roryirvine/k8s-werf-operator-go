package registry

import (
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name                string
		consecutiveFailures int32
		expectedDuration    time.Duration
		allowEqualOrGreater bool
	}{
		{
			name:                "0 failures: 30 seconds",
			consecutiveFailures: 0,
			expectedDuration:    30 * time.Second,
		},
		{
			name:                "1 failure: 1 minute",
			consecutiveFailures: 1,
			expectedDuration:    1 * time.Minute,
		},
		{
			name:                "2 failures: 2 minutes",
			consecutiveFailures: 2,
			expectedDuration:    2 * time.Minute,
		},
		{
			name:                "3 failures: 4 minutes",
			consecutiveFailures: 3,
			expectedDuration:    4 * time.Minute,
		},
		{
			name:                "4 failures: 8 minutes",
			consecutiveFailures: 4,
			expectedDuration:    8 * time.Minute,
		},
		{
			name:                "5 failures: 8 minutes (capped)",
			consecutiveFailures: 5,
			expectedDuration:    8 * time.Minute,
		},
		{
			name:                "10 failures: 8 minutes (capped)",
			consecutiveFailures: 10,
			expectedDuration:    8 * time.Minute,
		},
		{
			name:                "negative failures: 30 seconds (treated as 0)",
			consecutiveFailures: -5,
			expectedDuration:    30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBackoff(tt.consecutiveFailures)
			if result != tt.expectedDuration {
				t.Errorf("CalculateBackoff(%d) = %v, want %v", tt.consecutiveFailures, result, tt.expectedDuration)
			}
		})
	}
}

func TestAddJitter(t *testing.T) {
	// Test that jitter is within expected range (0-20%)
	baseInterval := 15 * time.Minute

	for i := 0; i < 100; i++ {
		result := AddJitter(baseInterval)

		// Minimum: baseInterval (0% jitter)
		minExpected := baseInterval
		// Maximum: baseInterval + 20% of baseInterval = baseInterval * 1.2
		maxExpected := baseInterval + (baseInterval / 5)

		if result < minExpected || result > maxExpected {
			t.Errorf("AddJitter(%v) = %v, want between %v and %v", baseInterval, result, minExpected, maxExpected)
		}
	}
}

func TestAddJitterSmallInterval(t *testing.T) {
	// Test with very small interval to ensure no rounding errors
	baseInterval := 1 * time.Second
	minExpected := baseInterval
	maxExpected := baseInterval + (baseInterval / 5) // 1.2 seconds

	for i := 0; i < 50; i++ {
		result := AddJitter(baseInterval)
		if result < minExpected || result > maxExpected {
			t.Errorf("AddJitter(%v) = %v, want between %v and %v", baseInterval, result, minExpected, maxExpected)
		}
	}
}

func TestAddJitterZeroInterval(t *testing.T) {
	// Test with zero interval - should remain zero
	result := AddJitter(0)
	if result != 0 {
		t.Errorf("AddJitter(0) = %v, want 0", result)
	}
}

func TestBackoffSequence(t *testing.T) {
	// Verify the full exponential backoff sequence
	expectedSequence := []time.Duration{
		30 * time.Second, // 0 failures
		1 * time.Minute,  // 1 failure
		2 * time.Minute,  // 2 failures
		4 * time.Minute,  // 3 failures
		8 * time.Minute,  // 4 failures
		8 * time.Minute,  // 5 failures (capped)
	}

	for failures, expected := range expectedSequence {
		result := CalculateBackoff(int32(failures))
		if result != expected {
			t.Errorf("Backoff sequence at failures=%d: got %v, want %v", failures, result, expected)
		}
	}
}
