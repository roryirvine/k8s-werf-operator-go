// Package controllers implements the WerfBundle reconciliation logic.
package controllers

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
	"github.com/werf/k8s-werf-operator-go/internal/converge"
	"github.com/werf/k8s-werf-operator-go/internal/registry"
)

const (
	finalizerName = "werf.io/finalizer"
)

// WerfBundleReconciler reconciles WerfBundle resources.
//
// Design notes on key patterns:
//
//  1. client.Client vs direct clientset:
//     We use client.Client (high-level abstraction from controller-runtime) instead of
//     direct Kubernetes clientset because:
//     - Automatic caching: Multiple Get() calls for same object don't hit API server
//     - Request batching: Efficient for bulk operations
//     - Mock-able: Easy to test with fake clients
//     - Consistent API: Works uniformly for all resource types
//
// 2. Get() vs List():
//
//   - Get(): Fetch single object by name/namespace. Used when we know the exact resource.
//     Example: r.Get(ctx, jobKey, &job) - we calculated the deterministic job name
//
//   - List(): Fetch multiple objects with optional label/field filtering.
//     Would use List() if we needed to find all jobs owned by a WerfBundle.
//
//     3. Concurrent reconciliation safety:
//     The controller-runtime manager prevents concurrent Reconcile() calls for the same
//     WerfBundle resource through internal locking. If two reconciliations somehow run
//     concurrently (e.g., different components triggering events), only one proceeds
//     at a time. This is handled transparently - we don't need to add locks.
type WerfBundleReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	RegistryClient registry.Client
}

// +kubebuilder:rbac:groups=werf.io,resources=werfbundles,verbs=get;list;watch
// +kubebuilder:rbac:groups=werf.io,resources=werfbundles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=create;get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get

// Reconcile implements the reconciliation loop for WerfBundle.
func (r *WerfBundleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the WerfBundle
	bundle := &werfv1alpha1.WerfBundle{}
	if err := r.Get(ctx, req.NamespacedName, bundle); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, could have been deleted
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch WerfBundle")
		return ctrl.Result{}, err
	}

	// Check if bundle is being deleted
	if bundle.DeletionTimestamp != nil {
		if controllerutil.ContainsFinalizer(bundle, finalizerName) {
			// Cleanup logic here if needed (none for Slice 1)
			controllerutil.RemoveFinalizer(bundle, finalizerName)
			if err := r.Update(ctx, bundle); err != nil {
				log.Error(err, "failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer is set
	if !controllerutil.ContainsFinalizer(bundle, finalizerName) {
		controllerutil.AddFinalizer(bundle, finalizerName)
		if err := r.Update(ctx, bundle); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Validate ServiceAccount exists before attempting to create Job
	saKey := types.NamespacedName{
		Name:      bundle.Spec.Converge.ServiceAccountName,
		Namespace: bundle.Namespace,
	}
	sa := &corev1.ServiceAccount{}
	if err := r.Get(ctx, saKey, sa); err != nil {
		if apierrors.IsNotFound(err) {
			errMsg := fmt.Sprintf("ServiceAccount %q not found in namespace %q",
				bundle.Spec.Converge.ServiceAccountName, bundle.Namespace)
			log.Info("ServiceAccount not found", "serviceAccount", saKey)
			if err := r.updateStatusFailed(ctx, bundle, errMsg); err != nil {
				log.Error(err, "failed to update status after SA validation")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get ServiceAccount")
		return ctrl.Result{}, err
	}

	// Poll registry for latest tag
	// Note: Authentication not yet implemented (Slice 2) - always uses nil for auth
	latestTag, err := r.RegistryClient.GetLatestTag(ctx, bundle.Spec.Registry.URL, nil)
	if err != nil {
		log.Error(err, "failed to get latest tag from registry")
		if err := r.updateStatusFailed(ctx, bundle, fmt.Sprintf("Registry error: %v", err)); err != nil {
			log.Error(err, "failed to update status after registry error")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// If no tag found, update status and wait
	if latestTag == "" {
		log.Info("no tags found in registry")
		if bundle.Status.Phase == "" || bundle.Status.Phase == werfv1alpha1.PhaseFailed {
			if err := r.updateStatusSyncing(ctx, bundle, ""); err != nil {
				log.Error(err, "failed to update status to Syncing")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// If latest tag matches what we already deployed, we're done
	if bundle.Status.LastAppliedTag == latestTag {
		if bundle.Status.Phase != werfv1alpha1.PhaseSynced {
			if err := r.updateStatusSynced(ctx, bundle, latestTag); err != nil {
				log.Error(err, "failed to update status to Synced")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// New tag found - ensure Job exists and monitor it
	log.Info("new tag found, ensuring converge job exists", "tag", latestTag)
	if err := r.updateStatusSyncing(ctx, bundle, latestTag); err != nil {
		log.Error(err, "failed to update status to Syncing")
		return ctrl.Result{}, err
	}

	// Build the Job spec
	jobBuilder := converge.NewBuilder(bundle).WithScheme(r.Scheme)
	jobSpec, err := jobBuilder.Build(latestTag)
	if err != nil {
		log.Error(err, "failed to build Job")
		r.updateStatusFailed(ctx, bundle, fmt.Sprintf("Failed to build Job: %v", err))
		return ctrl.Result{}, nil
	}

	// Check if Job already exists before creating
	jobKey := types.NamespacedName{
		Name:      jobSpec.Name,
		Namespace: jobSpec.Namespace,
	}
	existingJob := &batchv1.Job{}
	jobExists := r.Get(ctx, jobKey, existingJob) == nil

	if !jobExists {
		// Job doesn't exist, create it
		if err := r.Create(ctx, jobSpec); err != nil {
			log.Error(err, "failed to create Job")
			r.updateStatusFailed(ctx, bundle, fmt.Sprintf("Failed to create Job: %v", err))
			return ctrl.Result{}, nil
		}
		log.Info("Job created successfully", "jobName", jobSpec.Name)
		// Job just created, give it time to start
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Job exists, monitor it for completion
	log.Info("Job already exists, monitoring status", "jobName", existingJob.Name)

	// Check Job status
	if existingJob.Status.Succeeded > 0 {
		log.Info("Job succeeded, updating status to Synced", "tag", latestTag)
		r.updateStatusSynced(ctx, bundle, latestTag)
		return ctrl.Result{}, nil
	}

	if existingJob.Status.Failed > 0 {
		log.Info("Job failed", "jobName", existingJob.Name)
		if err := r.updateStatusFailed(ctx, bundle, "Job failed, see job logs for details"); err != nil {
			log.Error(err, "failed to update status after job failure")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Job is still running, requeue to check again
	log.Info("Job is still running, will recheck on next sync", "jobName", existingJob.Name)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// updateStatusSyncing sets status to Syncing and clears error.
// Returns error if status update fails so caller can decide to requeue.
func (r *WerfBundleReconciler) updateStatusSyncing(ctx context.Context, bundle *werfv1alpha1.WerfBundle, tag string) error {
	bundle.Status.Phase = werfv1alpha1.PhaseSyncing
	if tag != "" {
		bundle.Status.LastAppliedTag = tag
	}
	bundle.Status.LastErrorMessage = ""
	bundle.Status.LastSyncTime = nil

	return r.Status().Update(ctx, bundle)
}

// updateStatusSynced sets status to Synced with timestamp.
// Returns error if status update fails so caller can decide to requeue.
func (r *WerfBundleReconciler) updateStatusSynced(ctx context.Context, bundle *werfv1alpha1.WerfBundle, tag string) error {
	bundle.Status.Phase = werfv1alpha1.PhaseSynced
	bundle.Status.LastAppliedTag = tag
	bundle.Status.LastErrorMessage = ""
	now := metav1.Now()
	bundle.Status.LastSyncTime = &now

	return r.Status().Update(ctx, bundle)
}

// updateStatusFailed sets status to Failed with error message.
// Returns error if status update fails so caller can decide to requeue.
func (r *WerfBundleReconciler) updateStatusFailed(ctx context.Context, bundle *werfv1alpha1.WerfBundle, errMsg string) error {
	bundle.Status.Phase = werfv1alpha1.PhaseFailed
	bundle.Status.LastErrorMessage = errMsg

	return r.Status().Update(ctx, bundle)
}

// SetupWithManager sets up the controller with the Manager.
func (r *WerfBundleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Ignore status subresource updates to avoid infinite reconciliation
	pred := predicate.GenerationChangedPredicate{}

	return ctrl.NewControllerManagedBy(mgr).
		For(&werfv1alpha1.WerfBundle{}).
		Owns(&batchv1.Job{}).
		WithEventFilter(pred).
		Complete(r)
}
