// Package rbac provides RBAC validation utilities for the Werf operator.
package rbac

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateServiceAccountExists checks if a ServiceAccount exists in the specified namespace.
// Returns nil if the ServiceAccount exists, or an error if it doesn't exist or cannot be fetched.
// The error message includes the ServiceAccount name and namespace for easy debugging.
func ValidateServiceAccountExists(
	ctx context.Context,
	c client.Client,
	name string,
	namespace string,
) error {
	// TODO: Implementation in next step
	return nil
}
