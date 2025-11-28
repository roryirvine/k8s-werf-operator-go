// Package values provides utilities for resolving configuration values from ConfigMaps and Secrets.
package values

import (
	"context"
	"fmt"
	"strings"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolverImpl is the concrete implementation of the Resolver interface.
type ResolverImpl struct {
	client client.Client
}

// NewResolver creates a new Resolver with the given Kubernetes client.
func NewResolver(c client.Client) Resolver {
	return &ResolverImpl{client: c}
}

// ResolveValues fetches and merges values from all ValuesSource entries.
// Sources are processed in array order; later sources override earlier ones.
// Returns error if any required source is missing (unless marked Optional).
func (r *ResolverImpl) ResolveValues(
	ctx context.Context,
	sources []werfv1alpha1.ValuesSource,
	bundleNamespace string,
	targetNamespace string,
) (map[string]string, error) {
	allMaps := make([]map[string]string, 0, len(sources))

	for i, source := range sources {
		var data map[string]string
		var err error

		// Fetch from ConfigMap or Secret
		if source.ConfigMapRef != nil {
			name := source.ConfigMapRef.Name
			if name == "" {
				return nil, fmt.Errorf("source %d: ConfigMapRef name is empty", i)
			}
			data, err = fetchConfigMap(ctx, r.client, name, bundleNamespace, targetNamespace)
		} else if source.SecretRef != nil {
			name := source.SecretRef.Name
			if name == "" {
				return nil, fmt.Errorf("source %d: SecretRef name is empty", i)
			}
			data, err = fetchSecret(ctx, r.client, name, bundleNamespace, targetNamespace)
		} else {
			return nil, fmt.Errorf("source %d: neither ConfigMapRef nor SecretRef is set", i)
		}

		// Handle errors based on optional flag
		if err != nil {
			if source.Optional && isNotFoundError(err) {
				// Optional source not found - skip it
				continue
			}
			// Required source not found or other error - fail
			return nil, fmt.Errorf("source %d: %w", i, err)
		}

		allMaps = append(allMaps, data)
	}

	// Merge all maps in order
	return mergeMaps(allMaps...), nil
}

// isNotFoundError checks if the error indicates a resource was not found.
func isNotFoundError(err error) bool {
	// Check for Kubernetes NotFound errors
	if apierrors.IsNotFound(err) {
		return true
	}
	// Check for our custom "not found" errors from fetch functions
	// (These contain "not found" in the error message)
	if err != nil {
		return strings.Contains(err.Error(), "not found")
	}
	return false
}
