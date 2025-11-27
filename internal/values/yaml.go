// Package values provides utilities for resolving configuration values from ConfigMaps and Secrets.
package values

import (
	"fmt"
	"strconv"

	"sigs.k8s.io/yaml"
)

// parseYAML parses a YAML string and flattens it into a flat key-value map.
// Nested structures are flattened using dot notation (e.g., "foo.bar.baz").
// Arrays are indexed with brackets (e.g., "foo[0]", "foo[1]").
// All values are converted to strings.
func parseYAML(yamlData string) (map[string]string, error) {
	if yamlData == "" {
		return map[string]string{}, nil
	}

	var data interface{}
	if err := yaml.Unmarshal([]byte(yamlData), &data); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	result := make(map[string]string)
	flattenValue("", data, result)
	return result, nil
}

// flattenValue recursively flattens a value into the result map.
// prefix is the current key path (e.g., "foo.bar").
func flattenValue(prefix string, value interface{}, result map[string]string) {
	switch v := value.(type) {
	case map[string]interface{}:
		// Nested map - recurse into it
		for key, val := range v {
			newPrefix := key
			if prefix != "" {
				newPrefix = prefix + "." + key
			}
			flattenValue(newPrefix, val, result)
		}
	case []interface{}:
		// Array - index with brackets
		for i, val := range v {
			newPrefix := fmt.Sprintf("%s[%d]", prefix, i)
			flattenValue(newPrefix, val, result)
		}
	case string:
		result[prefix] = v
	case int:
		result[prefix] = strconv.Itoa(v)
	case int64:
		result[prefix] = strconv.FormatInt(v, 10)
	case float64:
		result[prefix] = strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		result[prefix] = strconv.FormatBool(v)
	case nil:
		result[prefix] = ""
	default:
		// Fallback for any other types
		result[prefix] = fmt.Sprintf("%v", v)
	}
}
