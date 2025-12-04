// Package values provides utilities for resolving configuration values from ConfigMaps and Secrets.
package values

import "strconv"

// escapeValue escapes a value string for safe use in CLI arguments by wrapping it in quotes
// and escaping special characters. This provides defense-in-depth protection even though the
// arguments are passed via exec.Command (which doesn't involve shell interpretation).
//
// The escaping ensures values are safe in case they're:
//   - Logged or displayed in error messages
//   - Used in contexts where shell interpretation might occur
//   - Passed to werf, which may have its own parsing requirements
//
// Uses strconv.Quote for robust handling of all special characters including:
//   - Spaces and tabs
//   - Double quotes and single quotes
//   - Newlines and other control characters
//   - Backslashes and escape sequences
//   - Equals signs and other shell metacharacters
//
// Examples:
//   - "hello" -> "\"hello\""
//   - "value with spaces" -> "\"value with spaces\""
//   - "say \"hello\"" -> "\"say \\\"hello\\\"\""
//   - "line1\nline2" -> "\"line1\\nline2\""
func escapeValue(value string) string {
	return strconv.Quote(value)
}
