// Package values provides utilities for resolving configuration values from ConfigMaps and Secrets.
package values

import "strings"

// escapeValue escapes special characters in a value string for use in Helm --set arguments.
// Werf uses Helm under the hood, so we follow Helm's escaping conventions.
//
// Helm requires backslash-escaping for these special characters:
//   - Backslash (\) - must be escaped first to avoid double-escaping
//   - Comma (,) - Helm's value separator in --set key1=val1,key2=val2
//   - Equals (=) - Helm's key=value separator
//   - Brackets ([, ]) - used in array notation like servers[0].name
//
// The format is key=value with NO surrounding quotes. Helm handles the value parsing.
//
// Examples:
//   - "hello" -> "hello" (no change)
//   - "value,with,commas" -> "value\,with\,commas"
//   - "key=value" -> "key\=value"
//   - "path\to\file" -> "path\\to\\file"
//   - "servers[0]" -> "servers\[0\]"
//
// References:
//   - https://helm.sh/docs/intro/using_helm/
//   - https://werf.io/documentation/v1.2/reference/cli/werf_converge.html
func escapeValue(value string) string {
	// Escape backslashes first to avoid double-escaping
	result := strings.ReplaceAll(value, `\`, `\\`)

	// Escape Helm special characters
	result = strings.ReplaceAll(result, ",", `\,`)
	result = strings.ReplaceAll(result, "=", `\=`)
	result = strings.ReplaceAll(result, "[", `\[`)
	result = strings.ReplaceAll(result, "]", `\]`)

	return result
}
