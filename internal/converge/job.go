// Package converge provides Kubernetes Job creation for werf converge operations.
package converge

import (
	"crypto/rand"
	"encoding/hex"
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

	// Calculate TTL for log retention based on configured retention days
	ttlSeconds := b.getLogRetentionSeconds()

	// Apply resource limits from spec or use defaults
	cpuLimit := b.getResourceLimit("cpu")
	memoryLimit := b.getResourceLimit("memory")
	cpuRequest := b.getResourceLimit("cpu")       // Requests match limits
	memoryRequest := b.getResourceLimit("memory") // Requests match limits

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
			TTLSecondsAfterFinished: ttlSeconds,
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

// jobName generates a unique name for the job with format: <bundle>-<tag-hash>-<uuid>.
// The tag hash is deterministic (enables duplicate detection), UUID ensures collision prevention.
// Uses 8 hex chars for both tag hash and UUID for readability.
func (b *Builder) jobName(tag string) string {
	// Generate tag hash (deterministic for duplicate detection)
	h := fnv.New32a()
	h.Write([]byte(tag))
	tagHash := fmt.Sprintf("%x", h.Sum32())[:8]

	// Generate random UUID (8 hex chars = 32 bits of randomness)
	uuidBytes := make([]byte, 4)
	if _, err := rand.Read(uuidBytes); err != nil {
		// Fallback to empty UUID if randomness fails (should never happen)
		return fmt.Sprintf("%s-%s", b.werf.Name, tagHash)
	}
	uuid := hex.EncodeToString(uuidBytes)

	// Kubernetes names must be 253 characters or less
	// Format: <bundle>-<tag-hash>-<uuid>
	fullName := fmt.Sprintf("%s-%s-%s", b.werf.Name, tagHash, uuid)
	if len(fullName) <= 253 {
		return fullName
	}

	// Truncate bundle name if needed to fit within 253 chars
	// Account for: bundle-hash-uuid = bundle-(8)-(8) with 2 hyphens
	maxBundleLen := 253 - len(tagHash) - len(uuid) - 2
	if maxBundleLen > 0 {
		return fmt.Sprintf("%s-%s-%s", b.werf.Name[:maxBundleLen], tagHash, uuid)
	}

	// Last resort: use hash and uuid only (very long bundle names)
	return fmt.Sprintf("%s-%s", tagHash, uuid)
}

// getLogRetentionSeconds returns the TTL in seconds for automatic job cleanup.
// Converts configured LogRetentionDays to seconds, or returns default of 7 days.
func (b *Builder) getLogRetentionSeconds() *int32 {
	var days int32 = 7 // default: 7 days

	// Check if custom retention is specified
	if b.werf.Spec.Converge.LogRetentionDays != nil && *b.werf.Spec.Converge.LogRetentionDays > 0 {
		days = *b.werf.Spec.Converge.LogRetentionDays
	}

	// Convert days to seconds: days * 24 hours/day * 60 min/hour * 60 sec/min
	ttlSeconds := days * 24 * 60 * 60
	return &ttlSeconds
}

// getResourceLimit returns the configured resource limit or a sensible default.
// resourceType should be "cpu" or "memory".
func (b *Builder) getResourceLimit(resourceType string) *resource.Quantity {
	var value string

	// Check if custom limits are specified
	if b.werf.Spec.Converge.ResourceLimits != nil {
		switch resourceType {
		case "cpu":
			if b.werf.Spec.Converge.ResourceLimits.CPU != "" {
				value = b.werf.Spec.Converge.ResourceLimits.CPU
			}
		case "memory":
			if b.werf.Spec.Converge.ResourceLimits.Memory != "" {
				value = b.werf.Spec.Converge.ResourceLimits.Memory
			}
		}
	}

	// Apply defaults if not specified
	if value == "" {
		switch resourceType {
		case "cpu":
			value = "1"
		case "memory":
			value = "1Gi"
		}
	}

	return mustParseResource(value)
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
