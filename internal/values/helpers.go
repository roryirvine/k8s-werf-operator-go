// Package values provides utilities for resolving configuration values from ConfigMaps and Secrets.
package values

import werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"

// GetTargetNamespace returns the target namespace from ConvergeConfig, or the bundle namespace if not specified.
// This implements the fallback behavior where targetNamespace defaults to bundleNamespace.
func GetTargetNamespace(convergeConfig *werfv1alpha1.ConvergeConfig, bundleNamespace string) string {
	if convergeConfig.TargetNamespace != "" {
		return convergeConfig.TargetNamespace
	}
	return bundleNamespace
}

// GenerateSetFlags converts a flat key-value map to werf CLI --set arguments.
// Each key-value pair becomes a --set flag: --set key=value
// Returns a slice of strings suitable for passing to werf converge command.
// Keys are returned in sorted order for deterministic output.
func GenerateSetFlags(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sortStrings(keys)

	// Generate --set flags
	flags := make([]string, 0, len(values)*2)
	for _, key := range keys {
		flags = append(flags, "--set")
		flags = append(flags, key+"="+values[key])
	}

	return flags
}

// sortStrings sorts a slice of strings in place using simple bubble sort.
func sortStrings(s []string) {
	n := len(s)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if s[j] > s[j+1] {
				s[j], s[j+1] = s[j+1], s[j]
			}
		}
	}
}
