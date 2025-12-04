// Package values provides utilities for resolving configuration values from ConfigMaps and Secrets.
package values

import (
	"context"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
)

// Resolver fetches and merges values from ValuesSource entries.
type Resolver interface {
	// ResolveValues fetches ConfigMaps/Secrets and merges them into a flat map.
	// Sources are processed in array order; later sources override earlier ones.
	// bundleNamespace is checked first (admin-controlled), then targetNamespace.
	// Returns error if any required source is missing.
	ResolveValues(
		ctx context.Context,
		sources []werfv1alpha1.ValuesSource,
		bundleNamespace string,
		targetNamespace string,
	) (map[string]string, error)
}
