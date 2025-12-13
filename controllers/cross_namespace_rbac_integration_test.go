package controllers

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
	testingutil "github.com/werf/k8s-werf-operator-go/internal/testing"
)

// TestCrossNamespaceDeployment_RBACHelpers demonstrates how RBAC helpers simplify
// setting up complex cross-namespace test scenarios.
//
// This integration test shows:
// - Creating isolated namespaces for bundle and target deployment
// - Setting up proper RBAC using helper functions
// - Verifying RBAC setup enables cross-namespace deployments
func TestCrossNamespaceDeployment_RBACHelpers(t *testing.T) {
	ctx := context.Background()

	// Setup: Create target namespace with deploy permissions using helper
	// This demonstrates how helpers reduce boilerplate compared to manual setup
	targetNs, targetSa, err := testingutil.CreateNamespaceWithDeployPermissions(
		ctx,
		testk8sClient,
		"my-app-prod",
		"werf-deployer",
	)
	if err != nil {
		t.Fatalf("failed to create target namespace with RBAC: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, targetNs) }()
	defer func() { _ = testk8sClient.Delete(ctx, targetSa) }()

	// Verify that the helper successfully created all RBAC resources
	if targetNs == nil || targetNs.Name != "my-app-prod" {
		t.Errorf("expected namespace 'my-app-prod', got %v", targetNs)
	}

	if targetSa == nil || targetSa.Name != "werf-deployer" {
		t.Errorf("expected ServiceAccount 'werf-deployer', got %v", targetSa)
	}

	// Verify Role was created with proper permissions
	roleKey := client.ObjectKey{Name: "werf-deployer-role", Namespace: "my-app-prod"}
	role := &rbacv1.Role{}
	if err := testk8sClient.Get(ctx, roleKey, role); err != nil {
		t.Fatalf("expected role to exist: %v", err)
	}

	if len(role.Rules) == 0 {
		t.Error("expected role to have permission rules")
	}

	// Verify RoleBinding was created, linking SA to Role
	rb := &rbacv1.RoleBinding{}
	rbName := targetSa.Name + "-" + role.Name
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: rbName, Namespace: "my-app-prod"}, rb); err != nil {
		t.Fatalf("expected rolebinding to exist: %v", err)
	}

	// Demonstrate that RBAC helpers provide complete setup in one call
	// instead of manually creating namespace, SA, role, and RoleBinding separately
	if rb.RoleRef.Name != role.Name {
		t.Error("RoleBinding does not reference the correct Role")
	}
	if len(rb.Subjects) != 1 || rb.Subjects[0].Name != targetSa.Name {
		t.Error("RoleBinding does not have correct subject")
	}
}

// TestCrossNamespaceDeployment_MissingServiceAccount demonstrates that RBAC helpers
// enable testing error cases - when ServiceAccount is missing, deployment should fail.
func TestCrossNamespaceDeployment_MissingServiceAccount(t *testing.T) {
	ctx := context.Background()

	// Setup: Create target namespace without ServiceAccount
	targetNs, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "app-missing-sa")
	if err != nil {
		t.Fatalf("failed to create target namespace: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, targetNs) }()

	// Create WerfBundle pointing to non-existent ServiceAccount
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cross-ns-missing-sa",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				TargetNamespace:    "app-missing-sa",
				ServiceAccountName: "nonexistent-sa",
			},
		},
	}

	if err := testk8sClient.Create(ctx, bundle); err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, bundle) }()

	fakeReg := NewFakeRegistry()
	fakeReg.SetTags("ghcr.io/test/bundle", []string{"v1.0.0"})

	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
		Clientset:      testK8sClientset,
	}

	// Reconcile: Should fail due to missing ServiceAccount
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "cross-ns-missing-sa",
			Namespace: "default",
		},
	}

	_, err = reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Verify: No Job should be created
	jobs := &batchv1.JobList{}
	listOpts := &client.ListOptions{Namespace: "app-missing-sa"}
	if err := testk8sClient.List(ctx, jobs, listOpts); err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}

	if len(jobs.Items) != 0 {
		t.Errorf("expected no jobs when SA missing, got %d", len(jobs.Items))
	}

	// Verify: Bundle status should be Failed
	updatedBundle := &werfv1alpha1.WerfBundle{}
	if err := testk8sClient.Get(ctx, req.NamespacedName, updatedBundle); err != nil {
		t.Fatalf("failed to get updated bundle: %v", err)
	}

	if updatedBundle.Status.Phase != werfv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed, got %s", updatedBundle.Status.Phase)
	}

	if updatedBundle.Status.LastErrorMessage == "" {
		t.Error("expected error message explaining SA is missing")
	}
}

// TestCrossNamespaceDeployment_CustomRBAC demonstrates using RBAC helpers with
// custom permission specifications for specialized deployment scenarios.
func TestCrossNamespaceDeployment_CustomRBAC(t *testing.T) {
	ctx := context.Background()

	// Setup: Create target namespace with custom minimal permissions
	// In a real scenario, you might give Jobs-only permissions, or specific to your workload
	customRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs"},
			Verbs:     []string{"create", "get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list"},
		},
	}

	targetNs, targetSa, err := testingutil.CreateNamespaceWithRBAC(
		ctx,
		testk8sClient,
		"custom-perms-app",
		"job-runner",
		customRules,
	)
	if err != nil {
		t.Fatalf("failed to create target namespace with custom RBAC: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, targetNs) }()
	defer func() { _ = testk8sClient.Delete(ctx, targetSa) }()

	// Verify custom Role was created with correct rules
	customRoleKey := client.ObjectKey{Name: "job-runner-role", Namespace: "custom-perms-app"}
	role := &rbacv1.Role{}
	if err := testk8sClient.Get(ctx, customRoleKey, role); err != nil {
		t.Fatalf("failed to get role: %v", err)
	}

	if len(role.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(role.Rules))
	}

	// Verify RoleBinding exists
	rb := &rbacv1.RoleBinding{}
	rbName := targetSa.Name + "-" + role.Name
	if err := testk8sClient.Get(ctx, client.ObjectKey{Name: rbName, Namespace: "custom-perms-app"}, rb); err != nil {
		t.Errorf("failed to get rolebinding: %v", err)
	}

	// Verify RoleBinding correctly references Role and SA
	if rb.RoleRef.Name != role.Name {
		t.Errorf("RoleBinding references wrong role")
	}
	if len(rb.Subjects) != 1 || rb.Subjects[0].Name != targetSa.Name {
		t.Errorf("RoleBinding has wrong subjects")
	}
}
