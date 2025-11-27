package values

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
		// Found in bundle namespace
		return cm.Data, nil
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
			// Found in target namespace
			return cm.Data, nil
		}

		// If error is not NotFound, propagate it
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get ConfigMap %q from namespace %q: %w",
				name, targetNamespace, err)
		}
	}

	// Not found in either namespace
	if targetNamespace != "" && targetNamespace != bundleNamespace {
		return nil, fmt.Errorf("ConfigMap %q not found in namespaces %q or %q",
			name, bundleNamespace, targetNamespace)
	}
	return nil, fmt.Errorf("ConfigMap %q not found in namespace %q",
		name, bundleNamespace)
}
