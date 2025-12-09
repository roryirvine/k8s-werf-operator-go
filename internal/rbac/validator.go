// Package rbac provides RBAC validation utilities for the Werf operator.
//
// This package focuses on pre-flight validation of deployment prerequisites to provide
// clear, immediate error messages before Job creation. It intentionally performs best-effort
// checks rather than comprehensive RBAC validation.
//
// Design Philosophy:
//
// We validate ServiceAccount existence but NOT permissions. This is a deliberate trade-off:
//
// What we check:
//   - ServiceAccount exists in target namespace
//
// What we don't check (and why):
//   - RoleBinding existence: Too complex, requires checking multiple binding types
//   - Permission adequacy: Would require simulating kubectl auth can-i for every operation
//   - Namespace existence: Job creation will fail fast with clear error if namespace missing
//
// This approach catches the most common configuration mistake (missing ServiceAccount)
// while avoiding the complexity and maintenance burden of comprehensive permission validation.
// If the ServiceAccount lacks proper permissions, the Job will fail with Kubernetes RBAC
// errors that clearly indicate the permission problem.
//
// This is a pre-flight check for better UX, not a security guarantee. Users are responsible
// for configuring proper RBAC permissions in their target namespaces.
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
