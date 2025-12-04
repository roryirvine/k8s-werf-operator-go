package values

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ErrNotFound indicates a ConfigMap or Secret was not found in any searched namespace.
var ErrNotFound = errors.New("resource not found")

// fetchConfigMap retrieves a ConfigMap from either the bundle namespace or target namespace.
// It searches the bundle namespace first (admin-controlled values), then the target namespace.
// Returns the ConfigMap's data as a string map, or an error if not found in either namespace.
func fetchConfigMap(
	ctx context.Context,
	c client.Client,
	name string,
	bundleNamespace string,
	targetNamespace string,
) (map[string]string, error) {
	// Try bundle namespace first (admin-controlled, takes precedence)
	cm := &corev1.ConfigMap{}
	bundleKey := types.NamespacedName{
		Name:      name,
		Namespace: bundleNamespace,
	}

	err := c.Get(ctx, bundleKey, cm)
	if err == nil {
		// Found in bundle namespace - parse and merge all YAML values
		return parseAndMergeConfigMapData(cm.Data)
	}

	// If error is not NotFound, propagate it (API error, permission issue, etc.)
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get ConfigMap %q from namespace %q: %w",
			name, bundleNamespace, err)
	}

	// Not found in bundle namespace, try target namespace if different
	if targetNamespace != "" && targetNamespace != bundleNamespace {
		targetKey := types.NamespacedName{
			Name:      name,
			Namespace: targetNamespace,
		}

		err = c.Get(ctx, targetKey, cm)
		if err == nil {
			// Found in target namespace - parse and merge all YAML values
			return parseAndMergeConfigMapData(cm.Data)
		}

		// If error is not NotFound, propagate it
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get ConfigMap %q from namespace %q: %w",
				name, targetNamespace, err)
		}
	}

	// Not found in either namespace
	if targetNamespace != "" && targetNamespace != bundleNamespace {
		return nil, fmt.Errorf("configMap %q not found in namespaces %q or %q: %w",
			name, bundleNamespace, targetNamespace, ErrNotFound)
	}
	return nil, fmt.Errorf("configMap %q not found in namespace %q: %w",
		name, bundleNamespace, ErrNotFound)
}

// fetchSecret retrieves a Secret from either the bundle namespace or target namespace.
// It searches the bundle namespace first (admin-controlled values), then the target namespace.
// Returns the Secret's data as a string map (decoded from base64), or an error if not found.
func fetchSecret(
	ctx context.Context,
	c client.Client,
	name string,
	bundleNamespace string,
	targetNamespace string,
) (map[string]string, error) {
	// Try bundle namespace first (admin-controlled, takes precedence)
	secret := &corev1.Secret{}
	bundleKey := types.NamespacedName{
		Name:      name,
		Namespace: bundleNamespace,
	}

	err := c.Get(ctx, bundleKey, secret)
	if err == nil {
		// Found in bundle namespace - parse and merge all YAML values
		return parseAndMergeSecretData(secret.Data)
	}

	// If error is not NotFound, propagate it (API error, permission issue, etc.)
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get Secret %q from namespace %q: %w",
			name, bundleNamespace, err)
	}

	// Not found in bundle namespace, try target namespace if different
	if targetNamespace != "" && targetNamespace != bundleNamespace {
		targetKey := types.NamespacedName{
			Name:      name,
			Namespace: targetNamespace,
		}

		err = c.Get(ctx, targetKey, secret)
		if err == nil {
			// Found in target namespace - parse and merge all YAML values
			return parseAndMergeSecretData(secret.Data)
		}

		// If error is not NotFound, propagate it
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get Secret %q from namespace %q: %w",
				name, targetNamespace, err)
		}
	}

	// Not found in either namespace
	if targetNamespace != "" && targetNamespace != bundleNamespace {
		return nil, fmt.Errorf("secret %q not found in namespaces %q or %q: %w",
			name, bundleNamespace, targetNamespace, ErrNotFound)
	}
	return nil, fmt.Errorf("secret %q not found in namespace %q: %w",
		name, bundleNamespace, ErrNotFound)
}

// secretDataToStringMap converts Secret.Data (map[string][]byte) to map[string]string.
// Secret data is already base64-decoded by the Kubernetes client.
func secretDataToStringMap(data map[string][]byte) map[string]string {
	result := make(map[string]string, len(data))
	for k, v := range data {
		result[k] = string(v)
	}
	return result
}

// parseAndMergeConfigMapData parses each ConfigMap value as YAML and merges them.
// Each key in the ConfigMap is treated as containing a YAML document.
// The YAML is flattened and all results are merged together.
func parseAndMergeConfigMapData(data map[string]string) (map[string]string, error) {
	maps := make([]map[string]string, 0, len(data))
	for key, yamlData := range data {
		parsed, err := parseYAML(yamlData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML from ConfigMap key %q: %w", key, err)
		}
		maps = append(maps, parsed)
	}
	return mergeMaps(maps...), nil
}

// parseAndMergeSecretData parses each Secret value as YAML and merges them.
// Each key in the Secret is treated as containing a YAML document.
// The YAML is flattened and all results are merged together.
func parseAndMergeSecretData(data map[string][]byte) (map[string]string, error) {
	stringData := secretDataToStringMap(data)
	return parseAndMergeConfigMapData(stringData)
}
