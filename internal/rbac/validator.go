// Package rbac provides RBAC validation utilities for the Werf operator.
package rbac

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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
	saKey := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	sa := &corev1.ServiceAccount{}
	if err := c.Get(ctx, saKey, sa); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("ServiceAccount '%s' not found in namespace '%s'", name, namespace)
		}
		return fmt.Errorf("failed to get ServiceAccount '%s' in namespace '%s': %w", name, namespace, err)
	}

	return nil
}
