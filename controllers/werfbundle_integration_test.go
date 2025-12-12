// Package controllers provides integration tests for WerfBundle reconciliation.
// These tests verify the full end-to-end flow: WerfBundle creation → values resolution →
// cross-namespace deployment → Job creation → status updates.
//
// Integration tests complement unit tests by verifying component interactions:
// - Unit tests: verify individual functions work correctly in isolation
// - Integration tests: verify components work together through full reconciliation loops
//
// Key differences from unit tests:
// - Use real envtest Kubernetes API (not just mocked clients)
// - Test complete reconciliation flows, not just single functions
// - Verify WerfBundle status is updated correctly
// - Test error paths and recovery
// - Verify Job creation with correct namespace, RBAC, and values
//
// Test patterns used:
// 1. Setup: Create test resources (namespaces, ConfigMaps, Secrets, ServiceAccounts)
// 2. Execute: Trigger reconciliation via Reconcile() call
// 3. Verify: Assert on Job creation, namespace placement, --set flags, and status
//
// Helpers from preceding issues reduce boilerplate:
// - RBAC helpers (issue #19): CreateNamespaceWithDeployPermissions(), CreateTestServiceAccount()
// - Values helpers (issue #20): CreateTestConfigMapWithValues(), CreateTestSecretWithValues(), AssertJobSetFlagsEqual()
// - Test fixtures (issue #18): Pre-built YAML test data in testdata directories
package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
	testingutil "github.com/werf/k8s-werf-operator-go/internal/testing"
)

// testBundleNameForStep generates a unique bundle name using timestamp.
// This ensures each test gets a unique name, avoiding conflicts.
func testBundleNameForStep(stepName string) string {
	return fmt.Sprintf("test-bundle-%s-%d", stepName, time.Now().UnixNano())
}

// reconcileWerfBundle is a helper to execute reconciliation for a WerfBundle.
// Returns the reconciliation result and any error.
func reconcileWerfBundle(t *testing.T, ctx context.Context, bundleName, bundleNs string) (ctrl.Result, error) {
	t.Helper()

	// Create reconciler with dependencies
	fakeReg := NewFakeRegistry()
	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
		Clientset:      testK8sClientset,
	}

	// Prepare reconciliation request
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      bundleName,
			Namespace: bundleNs,
		},
	}

	// Trigger reconciliation
	return reconciler.Reconcile(ctx, req)
}

// getWerfBundle fetches a WerfBundle from the cluster.
func getWerfBundle(t *testing.T, ctx context.Context, name, namespace string) *werfv1alpha1.WerfBundle {
	t.Helper()
	bundle := &werfv1alpha1.WerfBundle{}
	err := testk8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, bundle)
	if err != nil {
		t.Fatalf("failed to get WerfBundle: %v", err)
	}
	return bundle
}

// getJobInNamespace fetches a Job by name from a specific namespace.
func getJobInNamespace(t *testing.T, ctx context.Context, name, namespace string) *batchv1.Job {
	t.Helper()
	job := &batchv1.Job{}
	err := testk8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, job)
	if err != nil {
		t.Fatalf("failed to get Job: %v", err)
	}
	return job
}

// jobExists checks if a Job exists in a given namespace.
func jobExists(t *testing.T, ctx context.Context, name, namespace string) bool {
	t.Helper()
	job := &batchv1.Job{}
	err := testk8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, job)
	return err == nil
}

// TestIntegration_ValuesFromSingleConfigMap_JobHasSetFlags verifies that a WerfBundle
// with a single ConfigMap source creates a Job with correct --set flags.
//
// This integration test verifies:
// - ConfigMap is fetched from bundle namespace
// - Values are parsed and flattened correctly
// - Job is created with --set flags for all values
// - WerfBundle status is updated to Syncing
//
// Test scenario:
// 1. Create ConfigMap "app-config" with app.name and app.replicas
// 2. Create WerfBundle referencing ConfigMap in ValuesFrom
// 3. Reconcile
// 4. Verify Job created with --set app.name=myapp --set app.replicas=3
// 5. Verify WerfBundle status is Syncing
func TestIntegration_ValuesFromSingleConfigMap_JobHasSetFlags(t *testing.T) {
	ctx := context.Background()
	bundleName := testBundleNameForStep("single-configmap")

	// Step 1: Create ConfigMap with test values using helper
	configMapValues := map[string]string{
		"app.name":     "myapp",
		"app.replicas": "3",
	}
	cm, err := testingutil.CreateTestConfigMapWithValues(ctx, testk8sClient, "default", "app-config", configMapValues)
	if err != nil {
		t.Fatalf("failed to create ConfigMap: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, cm) }()

	// Step 2: Create WerfBundle with ValuesFrom ConfigMap
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
				ValuesFrom: []werfv1alpha1.ValuesSource{
					{
						ConfigMapRef: &corev1.LocalObjectReference{Name: "app-config"},
					},
				},
			},
		},
	}
	err = testk8sClient.Create(ctx, bundle)
	if err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, bundle) }()

	// Step 3: Reconcile
	_, err = reconcileWerfBundle(t, ctx, bundleName, "default")
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	// Step 4: Verify Job created with correct --set flags
	job := getJobInNamespace(t, ctx, bundleName, "default")
	testingutil.AssertJobSetFlagsEqual(t, job, configMapValues)

	// Step 5: Verify WerfBundle status is Syncing
	updatedBundle := getWerfBundle(t, ctx, bundleName, "default")
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected status phase Syncing, got %v", updatedBundle.Status.Phase)
	}
}

// TestIntegration_ValuesFromConfigMapAndSecret_BothMerged verifies that a WerfBundle
// with multiple sources (ConfigMap and Secret) merges them correctly in Job --set flags.
//
// This integration test verifies:
// - ConfigMap and Secret are both fetched
// - Values from both sources are merged in array order
// - Job contains --set flags for all keys from both sources
// - WerfBundle status is updated to Syncing
//
// Test scenario:
// 1. Create ConfigMap with app.name and app.replicas
// 2. Create Secret with db.password and db.host
// 3. Create WerfBundle with ValuesFrom referencing both (ConfigMap first, then Secret)
// 4. Reconcile
// 5. Verify Job has all 4 --set flags (2 from ConfigMap, 2 from Secret)
// 6. Verify WerfBundle status is Syncing
func TestIntegration_ValuesFromConfigMapAndSecret_BothMerged(t *testing.T) {
	ctx := context.Background()
	bundleName := testBundleNameForStep("configmap-and-secret")

	// Step 1: Create ConfigMap with app configuration
	configMapValues := map[string]string{
		"app.name":     "myapp",
		"app.replicas": "3",
	}
	cm, err := testingutil.CreateTestConfigMapWithValues(ctx, testk8sClient, "default", "app-config", configMapValues)
	if err != nil {
		t.Fatalf("failed to create ConfigMap: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, cm) }()

	// Step 2: Create Secret with database credentials
	secretValues := map[string]string{
		"db.password": "secret123",
		"db.host":     "db.example.com",
	}
	secret, err := testingutil.CreateTestSecretWithValues(ctx, testk8sClient, "default", "db-secrets", secretValues)
	if err != nil {
		t.Fatalf("failed to create Secret: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, secret) }()

	// Step 3: Create WerfBundle with both ConfigMap and Secret sources
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
				ValuesFrom: []werfv1alpha1.ValuesSource{
					{
						ConfigMapRef: &corev1.LocalObjectReference{Name: "app-config"},
					},
					{
						SecretRef: &corev1.LocalObjectReference{Name: "db-secrets"},
					},
				},
			},
		},
	}
	err = testk8sClient.Create(ctx, bundle)
	if err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, bundle) }()

	// Step 4: Reconcile
	_, err = reconcileWerfBundle(t, ctx, bundleName, "default")
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	// Step 5: Verify Job has all values from both sources merged
	job := getJobInNamespace(t, ctx, bundleName, "default")
	expectedValues := map[string]string{
		"app.name":     "myapp",
		"app.replicas": "3",
		"db.password":  "secret123",
		"db.host":      "db.example.com",
	}
	testingutil.AssertJobSetFlagsEqual(t, job, expectedValues)

	// Step 6: Verify WerfBundle status is Syncing
	updatedBundle := getWerfBundle(t, ctx, bundleName, "default")
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected status phase Syncing, got %v", updatedBundle.Status.Phase)
	}
}

// TestIntegration_ValuesFromMultipleSources_LaterSourceWins verifies that when multiple
// sources have overlapping keys, the later source's value wins (last-wins precedence).
//
// This integration test verifies:
// - Multiple ConfigMaps can provide values for the same keys
// - When keys overlap, later source wins
// - Job contains --set flags with values from the later source
// - WerfBundle status is updated to Syncing
//
// Test scenario:
// 1. Create "base-config" ConfigMap with app.environment=dev
// 2. Create "override-config" ConfigMap with app.environment=prod (same key, different value)
// 3. Create WerfBundle with both in ValuesFrom (base first, override second)
// 4. Reconcile
// 5. Verify Job has --set app.environment=prod (from override, not base)
// 6. Verify WerfBundle status is Syncing
//
// This demonstrates the merge precedence rule: sources are merged in array order,
// with later values overriding earlier ones for the same key.
func TestIntegration_ValuesFromMultipleSources_LaterSourceWins(t *testing.T) {
	ctx := context.Background()
	bundleName := testBundleNameForStep("precedence-override")

	// Step 1: Create "base" ConfigMap with initial values
	baseValues := map[string]string{
		"app.environment": "dev",
		"app.debug":       "false",
	}
	baseCM, err := testingutil.CreateTestConfigMapWithValues(ctx, testk8sClient, "default", "base-config", baseValues)
	if err != nil {
		t.Fatalf("failed to create base ConfigMap: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, baseCM) }()

	// Step 2: Create "override" ConfigMap with overlapping key (same app.environment)
	// This represents environment-specific overrides (e.g., prod overrides dev)
	overrideValues := map[string]string{
		"app.environment": "prod",
		"app.replicas":    "5",
	}
	overrideCM, err := testingutil.CreateTestConfigMapWithValues(ctx, testk8sClient, "default", "override-config", overrideValues)
	if err != nil {
		t.Fatalf("failed to create override ConfigMap: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, overrideCM) }()

	// Step 3: Create WerfBundle with both ConfigMaps (base first, override second)
	// This means override-config values take precedence over base-config values
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
				ValuesFrom: []werfv1alpha1.ValuesSource{
					{
						ConfigMapRef: &corev1.LocalObjectReference{Name: "base-config"},
					},
					{
						ConfigMapRef: &corev1.LocalObjectReference{Name: "override-config"},
					},
				},
			},
		},
	}
	err = testk8sClient.Create(ctx, bundle)
	if err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, bundle) }()

	// Step 4: Reconcile
	_, err = reconcileWerfBundle(t, ctx, bundleName, "default")
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	// Step 5: Verify Job has correct precedence: later source wins for overlapping keys
	job := getJobInNamespace(t, ctx, bundleName, "default")
	expectedValues := map[string]string{
		"app.environment": "prod",     // From override (later source)
		"app.debug":       "false",    // From base (no override)
		"app.replicas":    "5",        // From override
	}
	testingutil.AssertJobSetFlagsEqual(t, job, expectedValues)

	// Step 6: Verify WerfBundle status is Syncing
	updatedBundle := getWerfBundle(t, ctx, bundleName, "default")
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected status phase Syncing, got %v", updatedBundle.Status.Phase)
	}
}
