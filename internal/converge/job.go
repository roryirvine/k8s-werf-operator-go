// Package converge provides Kubernetes Job creation for werf converge operations.
package converge

import (
	"fmt"
	"hash/fnv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
)

// Builder creates Kubernetes Jobs for werf converge operations.
type Builder struct {
	werf   *werfv1alpha1.WerfBundle
	scheme *runtime.Scheme
}

// NewBuilder creates a new Job builder for a WerfBundle.
func NewBuilder(bundle *werfv1alpha1.WerfBundle) *Builder {
	return &Builder{werf: bundle, scheme: nil}
}

// WithScheme sets the scheme for controller reference generation.
func (b *Builder) WithScheme(scheme *runtime.Scheme) *Builder {
	b.scheme = scheme
	return b
}

// Build creates a Kubernetes Job spec for werf converge.
// The job name is deterministic based on bundle and tag to enable idempotency.
func (b *Builder) Build(tag string) (*batchv1.Job, error) {
	if b.werf == nil {
		return nil, fmt.Errorf("WerfBundle is nil")
	}

	jobName := b.jobName(tag)

	// Job retry policy: don't retry within the job, controller handles retries
	backoffLimit := int32(0)

	// Calculate TTL for log retention: default 7 days
	// TODO: Make this configurable via CRD in future phases
	ttlSeconds := int32(7 * 24 * 60 * 60) // 7 days in seconds

	// Apply resource limits with reasonable defaults
	// TODO: Make these configurable via CRD in future phases
	cpuRequest := mustParseResource("1")
	cpuLimit := mustParseResource("1")
	memoryRequest := mustParseResource("1Gi")
	memoryLimit := mustParseResource("1Gi")

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: b.werf.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "werf-operator",
				"app.kubernetes.io/instance":   b.werf.Name,
				"app.kubernetes.io/managed-by": "werf-operator",
				"werf.io/bundle":               b.werf.Name,
				"werf.io/tag":                  tag,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":       "werf-operator",
						"app.kubernetes.io/instance":   b.werf.Name,
						"app.kubernetes.io/managed-by": "werf-operator",
						"werf.io/bundle":               b.werf.Name,
						"werf.io/tag":                  tag,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: b.werf.Spec.Converge.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "werf",
							Image: "ghcr.io/werf/werf:latest",
							Args: []string{
								"converge",
								"--log-color=false",
								fmt.Sprintf("%s:%s", b.werf.Spec.Registry.URL, tag),
							},
							// Resource limits prevent runaway werf processes
							// Configurable via CRD in future phases
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *cpuRequest,
									corev1.ResourceMemory: *memoryRequest,
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *cpuLimit,
									corev1.ResourceMemory: *memoryLimit,
								},
							},
						},
					},
				},
			},
		},
	}

	// Set WerfBundle as owner of this Job for automatic cleanup
	if b.scheme != nil {
		if err := controllerutil.SetControllerReference(b.werf, job, b.scheme); err != nil {
			return nil, fmt.Errorf("failed to set controller reference: %w", err)
		}
	}

	return job, nil
}

// jobName generates a deterministic name for the job based on bundle name and tag.
// Using a hash ensures the name is stable and within Kubernetes naming constraints.
func (b *Builder) jobName(tag string) string {
	h := fnv.New32a()
	h.Write([]byte(tag))
	hash := fmt.Sprintf("%x", h.Sum32())[:8]

	// Kubernetes names must be 253 characters or less
	// Format: werf-<bundle>-<tag-hash>
	baseName := fmt.Sprintf("werf-%s", b.werf.Name)
	if len(baseName)+1+len(hash) <= 253 {
		return fmt.Sprintf("%s-%s", baseName, hash)
	}

	// Fallback: truncate bundle name if needed
	maxLen := 253 - len(hash) - 6 // "werf--" is 6 chars
	if maxLen > 0 {
		return fmt.Sprintf("werf-%s-%s", b.werf.Name[:maxLen], hash)
	}

	// Last resort: just use hash
	return fmt.Sprintf("werf-%s", hash)
}

// mustParseResource parses a resource string and panics on error.
// Safe to use for hardcoded resource values.
func mustParseResource(s string) *resource.Quantity {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		panic(fmt.Sprintf("invalid resource quantity %q: %v", s, err))
	}
	return &q
}
