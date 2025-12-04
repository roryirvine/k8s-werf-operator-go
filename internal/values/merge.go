// Package values provides utilities for resolving configuration values from ConfigMaps and Secrets.
package values

// mergeMaps merges multiple string maps in array order, with later maps overriding earlier ones.
// Uses a simple last-wins strategy for key conflicts - no deep merging.
// Returns a new map containing all merged key-value pairs.
func mergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
