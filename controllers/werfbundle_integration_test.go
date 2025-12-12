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
		"app.environment": "prod",  // From override (later source)
		"app.debug":       "false", // From base (no override)
		"app.replicas":    "5",     // From override
	}
	testingutil.AssertJobSetFlagsEqual(t, job, expectedValues)

	// Step 6: Verify WerfBundle status is Syncing
	updatedBundle := getWerfBundle(t, ctx, bundleName, "default")
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected status phase Syncing, got %v", updatedBundle.Status.Phase)
	}
}

// TestIntegration_ValuesFromMissingRequiredConfigMap_StatusFailed verifies that when
// a required ConfigMap source is missing, the reconciliation fails and WerfBundle status is Failed.
//
// This integration test verifies error handling:
// - Missing required ConfigMap is detected during reconciliation
// - NO Job is created
// - WerfBundle status is set to Failed
// - Error message mentions the missing ConfigMap
func TestIntegration_ValuesFromMissingRequiredConfigMap_StatusFailed(t *testing.T) {
	ctx := context.Background()
	bundleName := testBundleNameForStep("missing-required")

	// Create WerfBundle referencing non-existent ConfigMap (required, no optional flag)
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
						ConfigMapRef: &corev1.LocalObjectReference{Name: "nonexistent-config"},
						// No Optional field means it's required
					},
				},
			},
		},
	}
	err := testk8sClient.Create(ctx, bundle)
	if err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, bundle) }()

	// Reconcile
	_, err = reconcileWerfBundle(t, ctx, bundleName, "default")
	// Error is expected because required ConfigMap is missing

	// Verify NO Job was created
	if jobExists(t, ctx, bundleName, "default") {
		t.Error("expected no Job to be created when required ConfigMap is missing")
	}

	// Verify WerfBundle status is Failed with error message
	updatedBundle := getWerfBundle(t, ctx, bundleName, "default")
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseFailed {
		t.Errorf("expected status phase Failed, got %v", updatedBundle.Status.Phase)
	}
	if updatedBundle.Status.LastErrorMessage == "" {
		t.Error("expected error message when ConfigMap is missing")
	}
}

// TestIntegration_ValuesFromMissingOptionalSecret_JobCreated verifies that when
// an optional Secret source is missing, the reconciliation succeeds and skips that source.
//
// This integration test verifies optional source handling:
// - Missing optional Secret is skipped (not an error)
// - Job IS created with values from available sources
// - WerfBundle status is Syncing (not Failed)
func TestIntegration_ValuesFromMissingOptionalSecret_JobCreated(t *testing.T) {
	ctx := context.Background()
	bundleName := testBundleNameForStep("missing-optional")

	// Create ConfigMap (required source that exists)
	configMapValues := map[string]string{
		"app.name": "myapp",
	}
	cm, err := testingutil.CreateTestConfigMapWithValues(ctx, testk8sClient, "default", "app-config", configMapValues)
	if err != nil {
		t.Fatalf("failed to create ConfigMap: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, cm) }()

	// Create WerfBundle with both required ConfigMap and optional Secret (Secret doesn't exist)
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
						// Required (Optional is false by default)
					},
					{
						SecretRef: &corev1.LocalObjectReference{Name: "nonexistent-secret"},
						Optional:  true, // This one is optional
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

	// Reconcile
	_, err = reconcileWerfBundle(t, ctx, bundleName, "default")
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	// Verify Job WAS created (optional Secret missing is OK)
	job := getJobInNamespace(t, ctx, bundleName, "default")
	// Job should only have values from ConfigMap (Secret was skipped)
	testingutil.AssertJobSetFlagsEqual(t, job, configMapValues)

	// Verify WerfBundle status is Syncing (not Failed, since optional source was skipped)
	updatedBundle := getWerfBundle(t, ctx, bundleName, "default")
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected status phase Syncing, got %v", updatedBundle.Status.Phase)
	}
}

// TestIntegration_CrossNamespaceDeployment_JobInTargetNamespace verifies that a WerfBundle
// with TargetNamespace creates a Job in the target namespace, not the bundle namespace.
//
// This integration test verifies cross-namespace deployment:
// - Job is created in target namespace (not bundle namespace)
// - Job spec references the correct ServiceAccount
// - WerfBundle status is Syncing
func TestIntegration_CrossNamespaceDeployment_JobInTargetNamespace(t *testing.T) {
	ctx := context.Background()
	bundleName := testBundleNameForStep("cross-namespace")

	// Create target namespace with RBAC using helper
	targetNs, targetSa, err := testingutil.CreateNamespaceWithDeployPermissions(ctx, testk8sClient, "my-app-prod", "werf-deployer")
	if err != nil {
		t.Fatalf("failed to create target namespace with RBAC: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, targetNs) }()
	defer func() { _ = testk8sClient.Delete(ctx, targetSa) }()

	// Create WerfBundle in "default" with TargetNamespace="my-app-prod"
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
				TargetNamespace:    "my-app-prod",
				ServiceAccountName: "werf-deployer",
			},
		},
	}
	err = testk8sClient.Create(ctx, bundle)
	if err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, bundle) }()

	// Reconcile
	_, err = reconcileWerfBundle(t, ctx, bundleName, "default")
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	// Verify Job was created in target namespace (not "default")
	job := getJobInNamespace(t, ctx, bundleName, "my-app-prod")
	if job.Namespace != "my-app-prod" {
		t.Errorf("expected Job in namespace 'my-app-prod', got %v", job.Namespace)
	}

	// Verify Job PodSpec references correct ServiceAccount
	if job.Spec.Template.Spec.ServiceAccountName != "werf-deployer" {
		t.Errorf("expected ServiceAccountName 'werf-deployer', got %v", job.Spec.Template.Spec.ServiceAccountName)
	}

	// Verify WerfBundle status is Syncing
	updatedBundle := getWerfBundle(t, ctx, bundleName, "default")
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected status phase Syncing, got %v", updatedBundle.Status.Phase)
	}
}

// TestIntegration_CrossNamespaceMissingServiceAccount_ValidationFails verifies that when
// a required ServiceAccount doesn't exist in the target namespace, validation fails.
//
// This integration test verifies RBAC validation:
// - Target namespace exists but ServiceAccount doesn't
// - Reconciliation fails with validation error
// - No Job is created
// - WerfBundle status is Failed with error message
func TestIntegration_CrossNamespaceMissingServiceAccount_ValidationFails(t *testing.T) {
	ctx := context.Background()
	bundleName := testBundleNameForStep("missing-sa")

	// Create target namespace WITHOUT ServiceAccount
	targetNs, err := testingutil.CreateTestNamespace(ctx, testk8sClient, "app-no-sa")
	if err != nil {
		t.Fatalf("failed to create target namespace: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, targetNs) }()

	// Create WerfBundle pointing to non-existent ServiceAccount
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
				TargetNamespace:    "app-no-sa",
				ServiceAccountName: "nonexistent-sa", // This ServiceAccount doesn't exist
			},
		},
	}
	err = testk8sClient.Create(ctx, bundle)
	if err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, bundle) }()

	// Reconcile
	_, err = reconcileWerfBundle(t, ctx, bundleName, "default")
	// Error is expected because ServiceAccount doesn't exist

	// Verify NO Job was created
	if jobExists(t, ctx, bundleName, "app-no-sa") {
		t.Error("expected no Job when ServiceAccount is missing")
	}

	// Verify WerfBundle status is Failed with error message
	updatedBundle := getWerfBundle(t, ctx, bundleName, "default")
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseFailed {
		t.Errorf("expected status phase Failed, got %v", updatedBundle.Status.Phase)
	}
	if updatedBundle.Status.LastErrorMessage == "" {
		t.Error("expected error message when ServiceAccount is missing")
	}
}

// TestIntegration_NoTargetNamespace_JobInBundleNamespace verifies backward compatibility:
// when TargetNamespace is not set, Job is created in the bundle namespace.
//
// This integration test verifies backward compatibility:
// - WerfBundle without TargetNamespace field
// - Job created in bundle namespace (not a different namespace)
// - WerfBundle status is Syncing
func TestIntegration_NoTargetNamespace_JobInBundleNamespace(t *testing.T) {
	ctx := context.Background()
	bundleName := testBundleNameForStep("backward-compat")

	// Create WerfBundle WITHOUT TargetNamespace (legacy pattern)
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
				// No TargetNamespace - deploy in bundle namespace
				ServiceAccountName: "werf-converge",
			},
		},
	}
	err := testk8sClient.Create(ctx, bundle)
	if err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, bundle) }()

	// Reconcile
	_, err = reconcileWerfBundle(t, ctx, bundleName, "default")
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	// Verify Job was created in bundle namespace (default)
	job := getJobInNamespace(t, ctx, bundleName, "default")
	if job.Namespace != "default" {
		t.Errorf("expected Job in bundle namespace 'default', got %v", job.Namespace)
	}

	// Verify WerfBundle status is Syncing
	updatedBundle := getWerfBundle(t, ctx, bundleName, "default")
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected status phase Syncing, got %v", updatedBundle.Status.Phase)
	}
}

// TestIntegration_MultipleBundlesSameTarget_BothJobsCreated verifies that multiple
// WerfBundles can deploy to the same target namespace without conflicts.
//
// This integration test verifies multi-bundle scenarios:
// - Two WerfBundles created targeting the same namespace
// - Both Jobs created in target namespace
// - No naming conflicts or race conditions
// - Both have correct status
func TestIntegration_MultipleBundlesSameTarget_BothJobsCreated(t *testing.T) {
	ctx := context.Background()
	bundle1Name := testBundleNameForStep("multi-app1")
	bundle2Name := testBundleNameForStep("multi-app2")

	// Create target namespace with RBAC
	targetNs, targetSa, err := testingutil.CreateNamespaceWithDeployPermissions(ctx, testk8sClient, "shared-target", "werf-deployer")
	if err != nil {
		t.Fatalf("failed to create target namespace with RBAC: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, targetNs) }()
	defer func() { _ = testk8sClient.Delete(ctx, targetSa) }()

	// Create first WerfBundle
	bundle1 := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundle1Name,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/app1",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				TargetNamespace:    "shared-target",
				ServiceAccountName: "werf-deployer",
			},
		},
	}
	err = testk8sClient.Create(ctx, bundle1)
	if err != nil {
		t.Fatalf("failed to create bundle1: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, bundle1) }()

	// Create second WerfBundle
	bundle2 := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundle2Name,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/app2",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				TargetNamespace:    "shared-target",
				ServiceAccountName: "werf-deployer",
			},
		},
	}
	err = testk8sClient.Create(ctx, bundle2)
	if err != nil {
		t.Fatalf("failed to create bundle2: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, bundle2) }()

	// Reconcile both
	_, err = reconcileWerfBundle(t, ctx, bundle1Name, "default")
	if err != nil {
		t.Fatalf("reconciliation of bundle1 failed: %v", err)
	}

	_, err = reconcileWerfBundle(t, ctx, bundle2Name, "default")
	if err != nil {
		t.Fatalf("reconciliation of bundle2 failed: %v", err)
	}

	// Verify both Jobs created in shared target namespace
	job1 := getJobInNamespace(t, ctx, bundle1Name, "shared-target")
	if job1.Namespace != "shared-target" {
		t.Errorf("expected job1 in 'shared-target', got %v", job1.Namespace)
	}

	job2 := getJobInNamespace(t, ctx, bundle2Name, "shared-target")
	if job2.Namespace != "shared-target" {
		t.Errorf("expected job2 in 'shared-target', got %v", job2.Namespace)
	}

	// Verify both have correct status
	updatedBundle1 := getWerfBundle(t, ctx, bundle1Name, "default")
	if updatedBundle1.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected bundle1 status Syncing, got %v", updatedBundle1.Status.Phase)
	}

	updatedBundle2 := getWerfBundle(t, ctx, bundle2Name, "default")
	if updatedBundle2.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected bundle2 status Syncing, got %v", updatedBundle2.Status.Phase)
	}
}

// TestIntegration_ValuesInTargetNamespace_CrossNamespaceResolution verifies that values
// from ConfigMaps/Secrets in the target namespace are correctly resolved.
//
// This integration test verifies cross-namespace value resolution:
// - ConfigMap exists in target namespace (not bundle namespace)
// - Values are resolved from target namespace
// - Job created in target namespace with correct --set flags
// - Demonstrates namespace precedence (bundle ns checked first, then target ns)
func TestIntegration_ValuesInTargetNamespace_CrossNamespaceResolution(t *testing.T) {
	ctx := context.Background()
	bundleName := testBundleNameForStep("values-target-ns")

	// Create target namespace with RBAC
	targetNs, targetSa, err := testingutil.CreateNamespaceWithDeployPermissions(ctx, testk8sClient, "target-with-values", "werf-deployer")
	if err != nil {
		t.Fatalf("failed to create target namespace with RBAC: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, targetNs) }()
	defer func() { _ = testk8sClient.Delete(ctx, targetSa) }()

	// Create ConfigMap in TARGET namespace (not bundle namespace)
	configMapValues := map[string]string{
		"app.env":  "production",
		"app.tier": "backend",
	}
	cm, err := testingutil.CreateTestConfigMapWithValues(ctx, testk8sClient, "target-with-values", "prod-config", configMapValues)
	if err != nil {
		t.Fatalf("failed to create ConfigMap in target ns: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, cm) }()

	// Create WerfBundle in "default" referencing ConfigMap in target namespace
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
				TargetNamespace:    "target-with-values",
				ServiceAccountName: "werf-deployer",
				ValuesFrom: []werfv1alpha1.ValuesSource{
					{
						ConfigMapRef: &corev1.LocalObjectReference{Name: "prod-config"},
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

	// Reconcile
	_, err = reconcileWerfBundle(t, ctx, bundleName, "default")
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	// Verify Job created in target namespace with values from ConfigMap in target namespace
	job := getJobInNamespace(t, ctx, bundleName, "target-with-values")
	testingutil.AssertJobSetFlagsEqual(t, job, configMapValues)

	// Verify WerfBundle status is Syncing
	updatedBundle := getWerfBundle(t, ctx, bundleName, "default")
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected status phase Syncing, got %v", updatedBundle.Status.Phase)
	}
}

// TestIntegration_CrossNamespaceWithValues_FullFlow verifies the complete end-to-end flow:
// values from multiple namespaces + cross-namespace deployment + RBAC validation.
//
// This is the most complex integration scenario, combining:
// - Bundle namespace with ConfigMap (base configuration)
// - Target namespace with Secret (credentials)
// - Cross-namespace deployment to target namespace
// - RBAC validation (ServiceAccount exists)
// - Values resolution from both namespaces
// - Job created in target namespace with all values
//
// Test scenario:
// 1. Create bundle namespace with ConfigMap (app configuration)
// 2. Create target namespace with Secret (database credentials) and RBAC
// 3. Create WerfBundle with TargetNamespace and ValuesFrom (both sources)
// 4. Reconcile
// 5. Verify Job created in target namespace
// 6. Verify Job has --set flags from both ConfigMap and Secret
// 7. Verify WerfBundle status is Syncing
func TestIntegration_CrossNamespaceWithValues_FullFlow(t *testing.T) {
	ctx := context.Background()
	bundleName := testBundleNameForStep("full-flow")
	bundleNs := "bundle-ns"

	// Step 1: Create bundle namespace with ConfigMap
	bundleNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: bundleNs,
		},
	}
	err := testk8sClient.Create(ctx, bundleNamespace)
	if err != nil {
		t.Fatalf("failed to create bundle namespace: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, bundleNamespace) }()

	configMapValues := map[string]string{
		"app.name":     "myservice",
		"app.replicas": "3",
	}
	cm, err := testingutil.CreateTestConfigMapWithValues(ctx, testk8sClient, bundleNs, "app-config", configMapValues)
	if err != nil {
		t.Fatalf("failed to create ConfigMap in bundle namespace: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, cm) }()

	// Step 2: Create target namespace with Secret and RBAC
	targetNs, targetSa, err := testingutil.CreateNamespaceWithDeployPermissions(ctx, testk8sClient, "deploy-prod", "werf-deployer")
	if err != nil {
		t.Fatalf("failed to create target namespace with RBAC: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, targetNs) }()
	defer func() { _ = testk8sClient.Delete(ctx, targetSa) }()

	secretValues := map[string]string{
		"db.host":     "postgres.prod.svc.cluster.local",
		"db.password": "secret-password-123",
	}
	secret, err := testingutil.CreateTestSecretWithValues(ctx, testk8sClient, "deploy-prod", "db-creds", secretValues)
	if err != nil {
		t.Fatalf("failed to create Secret in target namespace: %v", err)
	}
	defer func() { _ = testk8sClient.Delete(ctx, secret) }()

	// Step 3: Create WerfBundle with cross-namespace deployment and values from both namespaces
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: bundleNs,
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/service",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				TargetNamespace:    "deploy-prod",
				ServiceAccountName: "werf-deployer",
				ValuesFrom: []werfv1alpha1.ValuesSource{
					{
						ConfigMapRef: &corev1.LocalObjectReference{Name: "app-config"},
						// References ConfigMap in bundle namespace (default behavior)
					},
					{
						SecretRef: &corev1.LocalObjectReference{Name: "db-creds"},
						// References Secret in target namespace (values resolver checks target first for name collisions)
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
	_, err = reconcileWerfBundle(t, ctx, bundleName, bundleNs)
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	// Step 5: Verify Job created in target namespace
	job := getJobInNamespace(t, ctx, bundleName, "deploy-prod")
	if job.Namespace != "deploy-prod" {
		t.Errorf("expected Job in target namespace 'deploy-prod', got %v", job.Namespace)
	}

	// Step 6: Verify Job has --set flags from both ConfigMap and Secret
	expectedValues := map[string]string{
		"app.name":     "myservice",
		"app.replicas": "3",
		"db.host":      "postgres.prod.svc.cluster.local",
		"db.password":  "secret-password-123",
	}
	testingutil.AssertJobSetFlagsEqual(t, job, expectedValues)

	// Step 7: Verify WerfBundle status is Syncing
	updatedBundle := getWerfBundle(t, ctx, bundleName, bundleNs)
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected status phase Syncing, got %v", updatedBundle.Status.Phase)
	}
}
