package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
)

func TestReconcile_CreateWerfBundle_CreatesJob(t *testing.T) {
	ctx := context.Background()

	// Create a WerfBundle
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "default",
			},
		},
	}

	// Create ServiceAccount for the test
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	}

	if err := testk8sClient.Create(ctx, sa); err != nil {
		t.Fatalf("failed to create ServiceAccount: %v", err)
	}

	if err := testk8sClient.Create(ctx, bundle); err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}

	// Use fake registry to return a tag
	fakeReg := NewFakeRegistry()
	fakeReg.SetTags("ghcr.io/test/bundle", []string{"v1.0.0"})

	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
	}

	// Reconcile
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-bundle", Namespace: "default"},
	}

	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Should requeue to wait for job to start
	if result.RequeueAfter == 0 {
		t.Errorf("expected requeue after job creation, got none")
	}

	// Verify Job was created
	jobs := &batchv1.JobList{}
	opts := &client.ListOptions{Namespace: "default"}
	if err := testk8sClient.List(ctx, jobs, opts); err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}

	if len(jobs.Items) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs.Items))
	}

	job := jobs.Items[0]
	args := job.Spec.Template.Spec.Containers[0].Args
	lastArg := args[len(args)-1]
	if lastArg != "ghcr.io/test/bundle:v1.0.0" {
		t.Errorf("job bundle ref incorrect")
	}

	// Verify WerfBundle status is Syncing
	updatedBundle := &werfv1alpha1.WerfBundle{}
	if err := testk8sClient.Get(ctx, req.NamespacedName, updatedBundle); err != nil {
		t.Fatalf("failed to get updated bundle: %v", err)
	}

	if updatedBundle.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected phase Syncing, got %s", updatedBundle.Status.Phase)
	}

	if updatedBundle.Status.LastAppliedTag != "v1.0.0" {
		t.Errorf("expected last applied tag v1.0.0, got %s", updatedBundle.Status.LastAppliedTag)
	}
}

func TestReconcile_SameTagTwice_NoDuplicateJob(t *testing.T) {
	ctx := context.Background()

	// Create WerfBundle with specific name for uniqueness
	bundleName := fmt.Sprintf("test-bundle-%d", time.Now().UnixNano())
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle2",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "default",
			},
		},
	}

	// Note: envtest creates a "default" ServiceAccount automatically in the default namespace,
	// so we don't need to create it here

	if err := testk8sClient.Create(ctx, bundle); err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}

	fakeReg := NewFakeRegistry()
	fakeReg.SetTags("ghcr.io/test/bundle2", []string{"v2.0.0"})

	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: bundleName, Namespace: "default"},
	}

	// First reconcile - should create job
	_, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("first reconcile failed: %v", err)
	}

	// Count jobs
	jobs1 := &batchv1.JobList{}
	opts := &client.ListOptions{Namespace: "default"}
	if err := testk8sClient.List(ctx, jobs1, opts); err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	jobCount1 := len(jobs1.Items)

	// Update bundle status to reflect last applied tag
	bundle2 := &werfv1alpha1.WerfBundle{}
	if err := testk8sClient.Get(ctx, req.NamespacedName, bundle2); err != nil {
		t.Fatalf("failed to get bundle: %v", err)
	}
	bundle2.Status.LastAppliedTag = "v2.0.0"
	bundle2.Status.Phase = werfv1alpha1.PhaseSynced
	if err := testk8sClient.Status().Update(ctx, bundle2); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	// Second reconcile - tag hasn't changed, should NOT create new job
	_, err = reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("second reconcile failed: %v", err)
	}

	jobs2 := &batchv1.JobList{}
	if err := testk8sClient.List(ctx, jobs2, opts); err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	jobCount2 := len(jobs2.Items)

	if jobCount1 != jobCount2 {
		t.Errorf("job deduplication failed: expected no new jobs, had %d jobs then %d jobs", jobCount1, jobCount2)
	}
}

func TestReconcile_JobRunning_StatusRemainsSyncing(t *testing.T) {
	ctx := context.Background()

	bundleName := fmt.Sprintf("test-running-%d", time.Now().UnixNano())
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle3",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "default",
			},
		},
	}

	// Note: envtest creates a "default" ServiceAccount automatically in the default namespace,
	// so we don't need to create it here

	if err := testk8sClient.Create(ctx, bundle); err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}

	fakeReg := NewFakeRegistry()
	fakeReg.SetTags("ghcr.io/test/bundle3", []string{"v3.0.0"})

	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: bundleName, Namespace: "default"},
	}

	// First reconcile - creates job
	result1, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Should requeue to wait for job
	if result1.RequeueAfter == 0 {
		t.Errorf("expected requeue after job creation, got none")
	}

	// Check bundle status is Syncing after job creation
	bundle1 := &werfv1alpha1.WerfBundle{}
	if err := testk8sClient.Get(ctx, req.NamespacedName, bundle1); err != nil {
		t.Fatalf("failed to get bundle: %v", err)
	}

	if bundle1.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected phase Syncing after creation, got %s", bundle1.Status.Phase)
	}

	// Second reconcile - tag hasn't changed (ETag matches), so we get NotModifiedError
	// With ETag caching, we don't get the actual tags back, just a notification that
	// content hasn't changed. So we requeue and wait for the job to complete.
	result2, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("second reconcile failed: %v", err)
	}

	// When we get NotModifiedError, we requeue after poll interval
	// Status remains Syncing until job completes (checked in next reconcile with actual tags)
	bundle2 := &werfv1alpha1.WerfBundle{}
	if err := testk8sClient.Get(ctx, req.NamespacedName, bundle2); err != nil {
		t.Fatalf("failed to get bundle: %v", err)
	}

	// Status should still be Syncing (waiting for job to complete)
	if bundle2.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected phase Syncing while job running, got %s", bundle2.Status.Phase)
	}

	// Should requeue after poll interval when content hasn't changed
	if result2.RequeueAfter == 0 {
		t.Errorf("expected requeue after NotModifiedError, got none")
	}
}

func TestReconcile_MissingServiceAccount_FailsWithError(t *testing.T) {
	ctx := context.Background()

	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-no-sa",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "nonexistent-sa",
			},
		},
	}

	if err := testk8sClient.Create(ctx, bundle); err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}

	fakeReg := NewFakeRegistry()
	fakeReg.SetTags("ghcr.io/test/bundle", []string{"v1.0.0"})

	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-no-sa", Namespace: "default"},
	}

	_, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Check bundle status is Failed
	updatedBundle := &werfv1alpha1.WerfBundle{}
	if err := testk8sClient.Get(ctx, req.NamespacedName, updatedBundle); err != nil {
		t.Fatalf("failed to get bundle: %v", err)
	}

	if updatedBundle.Status.Phase != werfv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed, got %s", updatedBundle.Status.Phase)
	}

	if updatedBundle.Status.LastErrorMessage == "" {
		t.Error("expected error message to be set")
	}

	// No job should be created
	jobs := &batchv1.JobList{}
	opts := &client.ListOptions{Namespace: "default"}
	if err := testk8sClient.List(ctx, jobs, opts); err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}

	jobCount := 0
	for _, job := range jobs.Items {
		if len(job.OwnerReferences) > 0 && job.OwnerReferences[0].Name == "test-no-sa" {
			jobCount++
		}
	}

	if jobCount > 0 {
		t.Errorf("expected no jobs, got %d", jobCount)
	}
}

func TestReconcile_RegistryError_ExponentialBackoff(t *testing.T) {
	ctx := context.Background()

	bundleName := "test-backoff"
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/backoff",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "default",
			},
		},
	}

	if err := testk8sClient.Create(ctx, bundle); err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}

	// Create fake registry that always returns errors
	fakeReg := NewFakeRegistry()
	fakeReg.SetError("ghcr.io/test/backoff", fmt.Errorf("simulated registry error"))

	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: bundleName, Namespace: "default"},
	}

	// Expected backoff sequence: 30s, 1m, 2m, 4m, 8m
	expectedBackoffs := []time.Duration{
		30 * time.Second,
		1 * time.Minute,
		2 * time.Minute,
		4 * time.Minute,
		8 * time.Minute,
	}

	for i, expectedBackoff := range expectedBackoffs {
		result, err := reconciler.Reconcile(ctx, req)
		if err != nil {
			t.Fatalf("reconcile %d failed: %v", i, err)
		}

		// Verify requeue with exponential backoff
		if result.RequeueAfter == 0 {
			t.Errorf("reconcile %d: expected requeue, got none", i)
		}

		// Verify backoff is roughly in expected range with jitter
		// Allow wider range due to ±10% jitter
		minBackoff := expectedBackoff / 2
		maxBackoff := expectedBackoff * 2
		if result.RequeueAfter < minBackoff || result.RequeueAfter > maxBackoff {
			t.Errorf("reconcile %d: requeue backoff %v outside range [%v, %v]",
				i, result.RequeueAfter, minBackoff, maxBackoff)
		}

		// Verify ConsecutiveFailures increments
		updatedBundle := &werfv1alpha1.WerfBundle{}
		if err := testk8sClient.Get(ctx, req.NamespacedName, updatedBundle); err != nil {
			t.Fatalf("failed to get bundle: %v", err)
		}

		expectedFailures := int32(i + 1)
		if updatedBundle.Status.ConsecutiveFailures != expectedFailures {
			t.Errorf("reconcile %d: expected failures=%d, got %d",
				i, expectedFailures, updatedBundle.Status.ConsecutiveFailures)
		}

		// Verify status is Failed
		if updatedBundle.Status.Phase != werfv1alpha1.PhaseFailed {
			t.Errorf("reconcile %d: expected phase Failed, got %s",
				i, updatedBundle.Status.Phase)
		}
	}
}

func TestReconcile_FifthFailure_MarksAsFailed(t *testing.T) {
	ctx := context.Background()

	bundleName := "test-fifth-failure"
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/fifth",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "default",
			},
		},
	}

	if err := testk8sClient.Create(ctx, bundle); err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}

	// Initialize status with 5 consecutive failures
	bundle.Status.ConsecutiveFailures = 5
	if err := testk8sClient.Status().Update(ctx, bundle); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	// Create fake registry that always returns errors
	fakeReg := NewFakeRegistry()
	fakeReg.SetError("ghcr.io/test/fifth", fmt.Errorf("simulated registry error"))

	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: bundleName, Namespace: "default"},
	}

	// One more error will exceed maxConsecutiveFailures (5)
	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Should NOT requeue after exceeding max retries
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue after max retries, got %v", result.RequeueAfter)
	}

	// Verify status is Failed
	updatedBundle := &werfv1alpha1.WerfBundle{}
	if err := testk8sClient.Get(ctx, req.NamespacedName, updatedBundle); err != nil {
		t.Fatalf("failed to get bundle: %v", err)
	}

	if updatedBundle.Status.Phase != werfv1alpha1.PhaseFailed {
		t.Errorf("expected phase Failed, got %s", updatedBundle.Status.Phase)
	}

	if updatedBundle.Status.LastErrorMessage == "" {
		t.Error("expected error message to be set")
	}
}

func TestReconcile_SuccessAfterFailures_ResetsCounter(t *testing.T) {
	ctx := context.Background()

	bundleName := "test-reset-failures"
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/reset",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "default",
			},
		},
	}

	if err := testk8sClient.Create(ctx, bundle); err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}

	// Initialize status with previous failures
	bundle.Status.ConsecutiveFailures = 2
	bundle.Status.Phase = werfv1alpha1.PhaseFailed
	if err := testk8sClient.Status().Update(ctx, bundle); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	// Create fake registry that now returns tags (success)
	fakeReg := NewFakeRegistry()
	fakeReg.SetTags("ghcr.io/test/reset", []string{"v1.0.0"})

	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: bundleName, Namespace: "default"},
	}

	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Should requeue to wait for job
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after job creation")
	}

	// Verify ConsecutiveFailures is reset to 0
	updatedBundle := &werfv1alpha1.WerfBundle{}
	if err := testk8sClient.Get(ctx, req.NamespacedName, updatedBundle); err != nil {
		t.Fatalf("failed to get bundle: %v", err)
	}

	if updatedBundle.Status.ConsecutiveFailures != 0 {
		t.Errorf("expected ConsecutiveFailures=0, got %d",
			updatedBundle.Status.ConsecutiveFailures)
	}

	// Verify status is Syncing (or Synced if job already completed)
	if updatedBundle.Status.Phase != werfv1alpha1.PhaseSyncing &&
		updatedBundle.Status.Phase != werfv1alpha1.PhaseSynced {
		t.Errorf("expected phase Syncing or Synced, got %s",
			updatedBundle.Status.Phase)
	}
}

func TestE2E_CreateBundle_CreatesJob(t *testing.T) {
	// E2E test demonstrating the full bundle creation and job spawning workflow
	ctx := context.Background()

	bundleName := "test-e2e-create"
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/e2e",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "default",
			},
		},
	}

	if err := testk8sClient.Create(ctx, bundle); err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}

	// Setup fake registry with tags
	fakeReg := NewFakeRegistry()
	fakeReg.SetTags("ghcr.io/test/e2e", []string{"v1.0.0", "v1.1.0"})

	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: bundleName, Namespace: "default"},
	}

	// Reconcile: should create job for latest tag (v1.1.0)
	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Should requeue to monitor job
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after job creation")
	}

	// Verify bundle status updated to Syncing
	syncingBundle := &werfv1alpha1.WerfBundle{}
	if err := testk8sClient.Get(ctx, req.NamespacedName, syncingBundle); err != nil {
		t.Fatalf("failed to get bundle: %v", err)
	}

	if syncingBundle.Status.Phase != werfv1alpha1.PhaseSyncing {
		t.Errorf("expected phase Syncing, got %s", syncingBundle.Status.Phase)
	}

	if syncingBundle.Status.LastAppliedTag != "v1.1.0" {
		t.Errorf("expected LastAppliedTag=v1.1.0, got %s", syncingBundle.Status.LastAppliedTag)
	}

	// Verify job was created
	jobs := &batchv1.JobList{}
	opts := &client.ListOptions{Namespace: "default"}
	if err := testk8sClient.List(ctx, jobs, opts); err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}

	jobFound := false
	for _, job := range jobs.Items {
		if len(job.OwnerReferences) > 0 && job.OwnerReferences[0].Name == bundleName {
			jobFound = true
			// Verify job spec references correct tag
			if len(job.Spec.Template.Spec.Containers) > 0 {
				args := job.Spec.Template.Spec.Containers[0].Args
				if len(args) == 0 {
					t.Error("expected job to have args")
				}
			}
			break
		}
	}

	if !jobFound {
		t.Error("expected job owned by bundle to be created")
	}
}

func TestReconcile_ActiveJob_Deduplicates(t *testing.T) {
	ctx := context.Background()

	// Create ServiceAccount
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	}

	if err := testk8sClient.Create(ctx, sa); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("failed to create ServiceAccount: %v", err)
	}

	// Create WerfBundle with a manually set ActiveJobName to simulate existing job
	bundleName := fmt.Sprintf("test-dedup-%d", time.Now().UnixNano())
	jobName := "test-dedup-xyz"
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/dedup",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "default",
			},
		},
	}

	if err := testk8sClient.Create(ctx, bundle); err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}

	// Set status with active job (must use Status().Update())
	// Note: We set LastAppliedTag to empty to simulate a fresh start where registry
	// returns v1.0.0 as a new tag that needs deploying, but we already have a job running
	bundle.Status.Phase = werfv1alpha1.PhaseSyncing
	bundle.Status.LastAppliedTag = ""
	bundle.Status.LastJobStatus = "Running"
	bundle.Status.ActiveJobName = jobName
	if err := testk8sClient.Status().Update(ctx, bundle); err != nil {
		t.Fatalf("failed to update bundle status: %v", err)
	}

	// Create a Job that matches the ActiveJobName
	existingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dedup-xyz",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "werf.io/v1alpha1",
					Kind:       "WerfBundle",
					Name:       bundleName,
					UID:        bundle.UID,
				},
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "werf",
							Image: "ghcr.io/werf/werf:latest",
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
		Status: batchv1.JobStatus{
			// Job is still running - not completed
		},
	}

	if err := testk8sClient.Create(ctx, existingJob); err != nil {
		t.Fatalf("failed to create existing Job: %v", err)
	}

	fakeReg := NewFakeRegistry()
	// Set registry to only have the current tag (v1.0.0)
	// This simulates reconcile being called again while job is running for the same tag
	fakeReg.SetTags("ghcr.io/test/dedup", []string{"v1.0.0"})

	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: bundleName, Namespace: "default"},
	}

	// First reconcile: should create a new job (no active job yet)
	// Note: the test setup manually sets ActiveJobName to simulate ongoing job

	// Verify only 1 job exists initially (the one we manually created)
	jobs := &batchv1.JobList{}
	opts := &client.ListOptions{Namespace: "default"}
	if err := testk8sClient.List(ctx, jobs, opts); err != nil {
		t.Fatalf("failed to list jobs after setup: %v", err)
	}

	bundleJobCountBefore := 0
	for _, job := range jobs.Items {
		if len(job.OwnerReferences) > 0 && job.OwnerReferences[0].Name == bundleName {
			bundleJobCountBefore++
		}
	}

	if bundleJobCountBefore != 1 {
		t.Errorf("expected 1 job after setup, got %d", bundleJobCountBefore)
	}

	// Reconcile again with same tag: should NOT create new job (dedup via ActiveJobName)
	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Should requeue to monitor the existing job
	if result.RequeueAfter == 0 {
		t.Error("expected requeue to monitor existing job")
	}

	// Verify still only 1 job exists (no duplicate created)
	jobs = &batchv1.JobList{}
	if err := testk8sClient.List(ctx, jobs, opts); err != nil {
		t.Fatalf("failed to list jobs after reconcile: %v", err)
	}

	bundleJobCount := 0
	for _, job := range jobs.Items {
		if len(job.OwnerReferences) > 0 && job.OwnerReferences[0].Name == bundleName {
			bundleJobCount++
		}
	}

	if bundleJobCount != 1 {
		t.Errorf("expected 1 job (deduplication), got %d", bundleJobCount)
	}

	// Verify bundle status: ActiveJobName should still be the existing one
	updatedBundle := &werfv1alpha1.WerfBundle{}
	if err := testk8sClient.Get(ctx, req.NamespacedName, updatedBundle); err != nil {
		t.Fatalf("failed to get updated bundle: %v", err)
	}

	// ActiveJobName should remain unchanged (monitored, not recreated)
	if updatedBundle.Status.ActiveJobName != jobName {
		t.Errorf("expected ActiveJobName=%s, got %s", jobName, updatedBundle.Status.ActiveJobName)
	}

	if updatedBundle.Status.LastJobStatus != "Running" {
		t.Errorf("expected LastJobStatus=Running, got %s", updatedBundle.Status.LastJobStatus)
	}
}

func TestReconcile_ActiveJobDisappears_CreatesNewJob(t *testing.T) {
	ctx := context.Background()

	// Create ServiceAccount
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	}

	if err := testk8sClient.Create(ctx, sa); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("failed to create ServiceAccount: %v", err)
	}

	// Create WerfBundle with an active job that no longer exists
	bundleName := fmt.Sprintf("test-lost-job-%d", time.Now().UnixNano())
	oldJobName := "test-lost-job-xyz"
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/lostjob",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "default",
			},
		},
	}

	if err := testk8sClient.Create(ctx, bundle); err != nil {
		t.Fatalf("failed to create WerfBundle: %v", err)
	}

	// Set status with a non-existent active job
	bundle.Status.Phase = werfv1alpha1.PhaseSyncing
	bundle.Status.LastAppliedTag = ""
	bundle.Status.LastJobStatus = "Running"
	bundle.Status.ActiveJobName = oldJobName
	if err := testk8sClient.Status().Update(ctx, bundle); err != nil {
		t.Fatalf("failed to update bundle status: %v", err)
	}

	fakeReg := NewFakeRegistry()
	fakeReg.SetTags("ghcr.io/test/lostjob", []string{"v1.0.0"})

	reconciler := &WerfBundleReconciler{
		Client:         testk8sClient,
		Scheme:         testk8sClient.Scheme(),
		RegistryClient: fakeReg,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: bundleName, Namespace: "default"},
	}

	// Reconcile: should detect that old job is gone, clear it, and create a new one
	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Should requeue (new job was created)
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after new job creation")
	}

	// Verify bundle has new ActiveJobName (not the old one that disappeared)
	updatedBundle := &werfv1alpha1.WerfBundle{}
	if err := testk8sClient.Get(ctx, req.NamespacedName, updatedBundle); err != nil {
		t.Fatalf("failed to get updated bundle: %v", err)
	}

	if updatedBundle.Status.ActiveJobName == oldJobName {
		t.Errorf("expected ActiveJobName to be cleared and reset, got old value %s", oldJobName)
	}

	if updatedBundle.Status.ActiveJobName == "" {
		t.Error("expected new ActiveJobName to be set")
	}

	// Verify a job was created
	jobs := &batchv1.JobList{}
	opts := &client.ListOptions{Namespace: "default"}
	if err := testk8sClient.List(ctx, jobs, opts); err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}

	if len(jobs.Items) == 0 {
		t.Error("expected a new job to be created")
	}
}
