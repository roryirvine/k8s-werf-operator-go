// Package testing provides utilities for writing tests of the Werf operator.
// This package contains helper functions for setting up Kubernetes resources (namespaces, ServiceAccounts, Roles, RoleBindings) in tests.
package testing

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateTestNamespace creates a new namespace in the Kubernetes API server.
// Useful for setting up isolated test environments.
//
// Example:
//
//	ns, err := testing.CreateTestNamespace(ctx, client, "test-namespace")
//	if err != nil {
//	    t.Fatalf("failed to create namespace: %v", err)
//	}
//	defer client.Delete(ctx, ns)
func CreateTestNamespace(ctx context.Context, c client.Client, name string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := c.Create(ctx, ns); err != nil {
		return nil, err
	}

	return ns, nil
}

// CreateTestServiceAccount creates a ServiceAccount in the specified namespace.
// The namespace must already exist.
//
// Example:
//
//	sa, err := testing.CreateTestServiceAccount(ctx, client, "test-ns", "test-sa")
//	if err != nil {
//	    t.Fatalf("failed to create ServiceAccount: %v", err)
//	}
func CreateTestServiceAccount(ctx context.Context, c client.Client, namespace, name string) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := c.Create(ctx, sa); err != nil {
		return nil, err
	}

	return sa, nil
}

// CreateTestRole creates a Role with the specified PolicyRules in the given namespace.
// Permissions are specified via the rules parameter.
//
// Example:
//
//	rules := []rbacv1.PolicyRule{
//	    {
//	        APIGroups: []string{"apps"},
//	        Resources: []string{"deployments"},
//	        Verbs:     []string{"get", "list", "watch", "create", "update"},
//	    },
//	}
//	role, err := testing.CreateTestRole(ctx, client, "test-ns", "deployer", rules)
//	if err != nil {
//	    t.Fatalf("failed to create Role: %v", err)
//	}
func CreateTestRole(ctx context.Context, c client.Client, namespace, name string, rules []rbacv1.PolicyRule) (*rbacv1.Role, error) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Rules: rules,
	}

	if err := c.Create(ctx, role); err != nil {
		return nil, err
	}

	return role, nil
}

// CreateTestRoleBinding creates a RoleBinding that grants the specified Role to the specified ServiceAccount.
// Both the Role and ServiceAccount must already exist in the same namespace.
//
// Example:
//
//	rb, err := testing.CreateTestRoleBinding(ctx, client, "test-ns", "deployer", "test-sa")
//	if err != nil {
//	    t.Fatalf("failed to create RoleBinding: %v", err)
//	}
func CreateTestRoleBinding(ctx context.Context, c client.Client, namespace, roleName, saName string) (*rbacv1.RoleBinding, error) {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName + "-" + roleName,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      saName,
				Namespace: namespace,
			},
		},
	}

	if err := c.Create(ctx, rb); err != nil {
		return nil, err
	}

	return rb, nil
}

// CreateNamespaceWithRBAC creates a complete RBAC setup: namespace, ServiceAccount, Role, and RoleBinding.
// This is a convenience helper for the common case of setting up a namespace with a ServiceAccount
// that has specific permissions.
//
// Returns the created namespace and ServiceAccount. The test can delete these resources for cleanup.
//
// Example:
//
//	rules := []rbacv1.PolicyRule{
//	    {
//	        APIGroups: []string{"apps"},
//	        Resources: []string{"deployments"},
//	        Verbs:     []string{"*"},
//	    },
//	}
//	ns, sa, err := testing.CreateNamespaceWithRBAC(ctx, client, "deploy-ns", "deployer", rules)
//	if err != nil {
//	    t.Fatalf("failed to create namespace with RBAC: %v", err)
//	}
//	defer client.Delete(ctx, ns)
//	defer client.Delete(ctx, sa)
func CreateNamespaceWithRBAC(ctx context.Context, c client.Client, namespace, saName string, rules []rbacv1.PolicyRule) (*corev1.Namespace, *corev1.ServiceAccount, error) {
	// Create namespace
	ns, err := CreateTestNamespace(ctx, c, namespace)
	if err != nil {
		return nil, nil, err
	}

	// Create ServiceAccount
	sa, err := CreateTestServiceAccount(ctx, c, namespace, saName)
	if err != nil {
		// Clean up namespace if SA creation fails
		_ = c.Delete(ctx, ns)
		return nil, nil, err
	}

	// Create Role
	role, err := CreateTestRole(ctx, c, namespace, saName+"-role", rules)
	if err != nil {
		// Clean up previous resources if Role creation fails
		_ = c.Delete(ctx, sa)
		_ = c.Delete(ctx, ns)
		return nil, nil, err
	}

	// Create RoleBinding
	_, err = CreateTestRoleBinding(ctx, c, namespace, role.Name, sa.Name)
	if err != nil {
		// Clean up all previous resources if RoleBinding creation fails
		_ = c.Delete(ctx, role)
		_ = c.Delete(ctx, sa)
		_ = c.Delete(ctx, ns)
		return nil, nil, err
	}

	return ns, sa, nil
}

// CreateNamespaceWithDeployPermissions creates a namespace with a ServiceAccount that has standard permissions for deploying applications.
// This is a convenience helper for the common test scenario of setting up a target namespace for cross-namespace deployment.
//
// Includes permissions for: Deployments, StatefulSets, Services, ConfigMaps, Secrets, and Pods.
//
// Example:
//
//	ns, sa, err := testing.CreateNamespaceWithDeployPermissions(ctx, client, "my-app-prod", "werf-deployer")
//	if err != nil {
//	    t.Fatalf("failed to create namespace: %v", err)
//	}
//	defer client.Delete(ctx, ns)
func CreateNamespaceWithDeployPermissions(ctx context.Context, c client.Client, namespace, saName string) (*corev1.Namespace, *corev1.ServiceAccount, error) {
	// Standard permissions for deploying applications
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets", "daemonsets"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"services", "configmaps", "secrets"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}

	return CreateNamespaceWithRBAC(ctx, c, namespace, saName, rules)
}
