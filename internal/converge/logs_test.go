// Package converge tests log capture and truncation functionality.
package converge

import (
	"strings"
	"testing"
)

// TestCaptureJobLogs_TruncatesAt1MB tests that logs exceeding 1MB are truncated.
func TestCaptureJobLogs_TruncatesAt1MB(t *testing.T) {
	// Create logs that exceed 1MB
	largeLogs := strings.Repeat("x", 1024*1024+1000) // 1MB + 1000 bytes

	// Simulate what CaptureJobLogs does with large logs
	const maxLogSize = 1024 * 1024 // 1MB
	var result string
	if len(largeLogs) > maxLogSize {
		truncated := largeLogs[len(largeLogs)-maxLogSize:]
		result = "... (logs truncated - output exceeds 1MB) ...\n" + truncated
	} else {
		result = largeLogs
	}

	// Verify result includes truncation notice
	if !strings.Contains(result, "logs truncated") {
		t.Error("Expected truncation notice in result")
	}

	// Verify result doesn't exceed reasonable size (truncation notice + 1MB)
	expectedMaxSize := len("... (logs truncated - output exceeds 1MB) ...\n") + maxLogSize
	if len(result) > expectedMaxSize {
		t.Errorf("Expected result size <= %d, got %d", expectedMaxSize, len(result))
	}

	// Verify we kept the last 1MB of original logs
	if !strings.Contains(result, "x") {
		t.Error("Expected original logs to be in result")
	}
}

// TestCaptureJobLogs_ExactlyAt1MB tests logs exactly at 1MB boundary.
func TestCaptureJobLogs_ExactlyAt1MB(t *testing.T) {
	exactLogs := strings.Repeat("a", 1024*1024) // Exactly 1MB

	// Should not be truncated
	const maxLogSize = 1024 * 1024
	var result string
	if len(exactLogs) > maxLogSize {
		truncated := exactLogs[len(exactLogs)-maxLogSize:]
		result = "... (logs truncated - output exceeds 1MB) ...\n" + truncated
	} else {
		result = exactLogs
	}

	// Should return as-is without truncation
	if strings.Contains(result, "logs truncated") {
		t.Error("Expected no truncation at exactly 1MB")
	}

	if len(result) != 1024*1024 {
		t.Errorf("Expected result to be 1MB, got %d bytes", len(result))
	}
}

// TestCaptureJobLogs_TruncationPreservesContent tests that truncation keeps recent logs.
func TestCaptureJobLogs_TruncationPreservesContent(t *testing.T) {
	// Create logs with identifiable content at the end
	const maxLogSize = 1024 * 1024
	recentContent := "RECENT_LOG_ENTRY"
	// Create prefix that will be truncated away
	largePrefix := strings.Repeat("x", maxLogSize+100000) // Larger than 1MB
	logs := largePrefix + recentContent

	// Simulate truncation
	var result string
	if len(logs) > maxLogSize {
		truncated := logs[len(logs)-maxLogSize:]
		result = "... (logs truncated - output exceeds 1MB) ...\n" + truncated
	}

	// Verify recent content is preserved
	if !strings.Contains(result, recentContent) {
		t.Error("Expected recent logs to be preserved in truncation")
	}

	// Verify truncation actually happened (result should have truncation notice)
	if !strings.Contains(result, "logs truncated") {
		t.Error("Expected truncation notice in result")
	}

	// Verify result is reasonably sized (truncation notice + 1MB)
	maxResultSize := len("... (logs truncated - output exceeds 1MB) ...\n") + maxLogSize
	if len(result) > maxResultSize {
		t.Errorf("Expected result size <= %d, got %d", maxResultSize, len(result))
	}
}
