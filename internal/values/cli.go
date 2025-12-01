// Package values provides utilities for resolving configuration values from ConfigMaps and Secrets.
package values

import "strconv"

// escapeValue escapes a value string for safe use in CLI arguments.
func escapeValue(value string) string {
	return strconv.Quote(value)
}
