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
			r.updateStatusFailed(ctx, bundle, errMsg)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get ServiceAccount")
		return ctrl.Result{}, err
	}

	// Poll registry for latest tag
	latestTag, err := r.RegistryClient.GetLatestTag(ctx, bundle.Spec.Registry.URL, nil)
	if err != nil {
		log.Error(err, "failed to get latest tag from registry")
		r.updateStatusFailed(ctx, bundle, fmt.Sprintf("Registry error: %v", err))
		return ctrl.Result{}, nil
	}

	// If no tag found, update status and wait
	if latestTag == "" {
		log.Info("no tags found in registry")
		if bundle.Status.Phase == "" || bundle.Status.Phase == werfv1alpha1.PhaseFailed {
			r.updateStatusSyncing(ctx, bundle, "")
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// If latest tag matches what we already deployed, we're done
	if bundle.Status.LastAppliedTag == latestTag {
		if bundle.Status.Phase != werfv1alpha1.PhaseSynced {
			r.updateStatusSynced(ctx, bundle, latestTag)
		}
		return ctrl.Result{}, nil
	}

	// New tag found - create a Job to converge
	log.Info("new tag found, creating converge job", "tag", latestTag)
	r.updateStatusSyncing(ctx, bundle, latestTag)

	jobBuilder := converge.NewBuilder(bundle).WithScheme(r.Scheme)
	job, err := jobBuilder.Build(latestTag)
	if err != nil {
		log.Error(err, "failed to build Job")
		r.updateStatusFailed(ctx, bundle, fmt.Sprintf("Failed to build Job: %v", err))
		return ctrl.Result{}, nil
	}

	// Create the Job
	if err := r.Create(ctx, job); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("Job already exists, checking status")
		} else {
			log.Error(err, "failed to create Job")
			r.updateStatusFailed(ctx, bundle, fmt.Sprintf("Failed to create Job: %v", err))
			return ctrl.Result{}, nil
		}
	}

	// Monitor the Job for completion
	jobKey := types.NamespacedName{
		Name:      job.Name,
		Namespace: job.Namespace,
	}
	createdJob := &batchv1.Job{}
	if err := r.Get(ctx, jobKey, createdJob); err != nil {
		log.Error(err, "failed to fetch created Job")
		return ctrl.Result{}, nil
	}

	// Check Job status
	if createdJob.Status.Succeeded > 0 {
		log.Info("Job succeeded, updating status to Synced", "tag", latestTag)
		r.updateStatusSynced(ctx, bundle, latestTag)
	} else if createdJob.Status.Failed > 0 {
		errMsg := fmt.Sprintf("Job failed, see job logs for details")
		log.Info("Job failed", "jobName", createdJob.Name)
		r.updateStatusFailed(ctx, bundle, errMsg)
	}
	// If job is still running, status is already set to Syncing, just requeue

	return ctrl.Result{}, nil
}

// updateStatusSyncing sets status to Syncing and clears error.
func (r *WerfBundleReconciler) updateStatusSyncing(ctx context.Context, bundle *werfv1alpha1.WerfBundle, tag string) {
	bundle.Status.Phase = werfv1alpha1.PhaseSyncing
	if tag != "" {
		bundle.Status.LastAppliedTag = tag
	}
	bundle.Status.LastErrorMessage = ""
	bundle.Status.LastSyncTime = nil

	if err := r.Status().Update(ctx, bundle); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to update status to Syncing")
	}
}

// updateStatusSynced sets status to Synced with timestamp.
func (r *WerfBundleReconciler) updateStatusSynced(ctx context.Context, bundle *werfv1alpha1.WerfBundle, tag string) {
	bundle.Status.Phase = werfv1alpha1.PhaseSynced
	bundle.Status.LastAppliedTag = tag
	bundle.Status.LastErrorMessage = ""
	now := metav1.Now()
	bundle.Status.LastSyncTime = &now

	if err := r.Status().Update(ctx, bundle); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to update status to Synced")
	}
}

// updateStatusFailed sets status to Failed with error message.
func (r *WerfBundleReconciler) updateStatusFailed(ctx context.Context, bundle *werfv1alpha1.WerfBundle, errMsg string) {
	bundle.Status.Phase = werfv1alpha1.PhaseFailed
	bundle.Status.LastErrorMessage = errMsg

	if err := r.Status().Update(ctx, bundle); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to update status to Failed")
	}
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
