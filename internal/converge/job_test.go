package converge

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
)

const (
	defaultMemory  = "1Gi"
	testBundleName = "test-app"
)

var testScheme = func() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(werfv1alpha1.AddToScheme(scheme))
	return scheme
}()

func TestBuilder_Build_ValidBundle(t *testing.T) {
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)
	job, err := builder.Build("v1.0.0")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job == nil {
		t.Fatal("job is nil")
	}

	// Verify job metadata
	if job.Namespace != "default" {
		t.Errorf("namespace: got %q, want %q", job.Namespace, "default")
	}

	if job.Name == "" {
		t.Error("job name is empty")
	}

	// Verify labels
	if job.Labels["app.kubernetes.io/instance"] != testBundleName {
		t.Errorf("instance label: got %q, want %q", job.Labels["app.kubernetes.io/instance"], testBundleName)
	}

	// Verify pod template
	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(job.Spec.Template.Spec.Containers))
	}

	container := job.Spec.Template.Spec.Containers[0]
	if container.Name != "werf" {
		t.Errorf("container name: got %q, want %q", container.Name, "werf")
	}

	// Verify args include the bundle URL and tag
	if len(container.Args) < 2 {
		t.Errorf("expected at least 2 args, got %d", len(container.Args))
	}

	if container.Args[0] != "converge" {
		t.Errorf("first arg: got %q, want %q", container.Args[0], "converge")
	}

	if container.Args[len(container.Args)-1] != "ghcr.io/test/bundle:v1.0.0" {
		t.Errorf("bundle arg: got %q, want %q", container.Args[len(container.Args)-1], "ghcr.io/test/bundle:v1.0.0")
	}

	// Verify service account
	if job.Spec.Template.Spec.ServiceAccountName != "werf-converge" {
		t.Errorf("service account: got %q, want %q", job.Spec.Template.Spec.ServiceAccountName, "werf-converge")
	}
}

func TestBuilder_Build_DeterministicName(t *testing.T) {
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBundleName,
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)

	// Build same job twice with same tag
	job1, _ := builder.Build("v1.0.0")
	job2, _ := builder.Build("v1.0.0")

	// Verify both names have the same format and bundle/tag hash parts
	// but different UUIDs (they're random)
	if job1.Name == job2.Name {
		t.Errorf("job names should differ due to random UUID: %q vs %q", job1.Name, job2.Name)
	}

	// Verify names have the correct format: bundle-hash-uuid
	// Extract components by splitting on hyphens (last two components are hash-uuid)
	parts1 := splitJobName(job1.Name)
	parts2 := splitJobName(job2.Name)

	// Bundle name should match
	if parts1.bundle != testBundleName || parts2.bundle != testBundleName {
		t.Errorf("bundle name mismatch in job names")
	}

	// Tag hash should match (same tag = same hash)
	if parts1.tagHash != parts2.tagHash {
		t.Errorf("tag hash should match for same tag: %q vs %q",
			parts1.tagHash, parts2.tagHash)
	}

	// UUIDs should differ (random for each job)
	if parts1.uuid == parts2.uuid {
		t.Errorf("UUIDs should differ between job creations")
	}

	// Build job with different tag
	job3, _ := builder.Build("v2.0.0")
	parts3 := splitJobName(job3.Name)

	// Different tag should produce different hash
	if parts1.tagHash == parts3.tagHash {
		t.Errorf("different tags should produce different hashes: %q vs %q",
			parts1.tagHash, parts3.tagHash)
	}
}

// jobNameParts represents the components of a job name
type jobNameParts struct {
	bundle  string
	tagHash string
	uuid    string
}

// splitJobName parses a job name in format: bundle-hash-uuid
func splitJobName(jobName string) jobNameParts {
	// The last two components (separated by hyphens) are hash and uuid (8 chars each)
	// Everything before that is the bundle name
	if len(jobName) < 17 { // Minimum: a-12345678-87654321 (1+1+8+1+8)
		return jobNameParts{bundle: jobName}
	}

	// UUID is last 8 characters after the last hyphen
	uuid := jobName[len(jobName)-8:]
	rest := jobName[:len(jobName)-9] // -1 for hyphen before uuid

	// Tag hash is next 8 characters before that
	if len(rest) < 8 {
		return jobNameParts{bundle: jobName}
	}

	tagHash := rest[len(rest)-8:]
	bundle := rest[:len(rest)-9] // -1 for hyphen before hash

	return jobNameParts{
		bundle:  bundle,
		tagHash: tagHash,
		uuid:    uuid,
	}
}

func TestBuilder_Build_OwnerReference(t *testing.T) {
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
			UID:       "test-uid-123",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)
	job, err := builder.Build("v1.0.0")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(job.OwnerReferences) == 0 {
		t.Fatal("expected owner references, got none")
	}

	ownerRef := job.OwnerReferences[0]
	if ownerRef.Kind != "WerfBundle" {
		t.Errorf("owner kind: got %q, want %q", ownerRef.Kind, "WerfBundle")
	}

	if ownerRef.Name != "test-app" {
		t.Errorf("owner name: got %q, want %q", ownerRef.Name, "test-app")
	}

	if ownerRef.UID != "test-uid-123" {
		t.Errorf("owner UID: got %q, want %q", ownerRef.UID, "test-uid-123")
	}

	trueVal := true
	if ownerRef.Controller == nil || *ownerRef.Controller != trueVal {
		t.Error("expected controller to be true")
	}
}

func TestBuilder_Build_ResourceLimits(t *testing.T) {
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)
	job, err := builder.Build("v1.0.0")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := job.Spec.Template.Spec.Containers[0]
	resources := container.Resources

	// Verify requests
	if resources.Requests == nil {
		t.Fatal("resource requests are nil")
	}

	cpuReq := resources.Requests[corev1.ResourceCPU]
	if cpuReq.String() != "1" {
		t.Errorf("CPU request: got %s, want 1", cpuReq.String())
	}

	memReq := resources.Requests[corev1.ResourceMemory]
	if memReq.String() != defaultMemory {
		t.Errorf("Memory request: got %s, want 1Gi", memReq.String())
	}

	// Verify limits
	if resources.Limits == nil {
		t.Fatal("resource limits are nil")
	}

	cpuLim := resources.Limits[corev1.ResourceCPU]
	if cpuLim.String() != "1" {
		t.Errorf("CPU limit: got %s, want 1", cpuLim.String())
	}

	memLim := resources.Limits[corev1.ResourceMemory]
	if memLim.String() != defaultMemory {
		t.Errorf("Memory limit: got %s, want 1Gi", memLim.String())
	}
}

func TestBuilder_Build_JobSpec(t *testing.T) {
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)
	job, _ := builder.Build("v1.0.0")

	// Verify job spec
	// Backoff limit of 0 means job doesn't retry; controller handles retries
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 0 {
		t.Error("expected backoff limit of 0")
	}

	// TTL should be 7 days for log retention
	expectedTTL := int32(7 * 24 * 60 * 60) // 604800 seconds
	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != expectedTTL {
		t.Errorf("expected TTL of %d seconds, got %d", expectedTTL, *job.Spec.TTLSecondsAfterFinished)
	}

	// Verify pod spec
	if job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("restart policy: got %v, want %v", job.Spec.Template.Spec.RestartPolicy, corev1.RestartPolicyNever)
	}
}

func TestBuilder_Build_NilBundle(t *testing.T) {
	builder := NewBuilder(nil)
	_, err := builder.Build("v1.0.0")

	if err == nil {
		t.Error("expected error for nil bundle, got nil")
	}
}

func TestBuilder_Build_CustomResourceLimits(t *testing.T) {
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-resources",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
				ResourceLimits: &werfv1alpha1.ResourceLimitsConfig{
					CPU:    "2",
					Memory: "2Gi",
				},
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)
	job, err := builder.Build("v1.0.0")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := job.Spec.Template.Spec.Containers[0]
	resources := container.Resources

	// Verify custom CPU limit is used
	cpuLim := resources.Limits[corev1.ResourceCPU]
	if cpuLim.String() != "2" {
		t.Errorf("CPU limit: got %s, want 2", cpuLim.String())
	}

	// Verify custom memory limit is used
	memLim := resources.Limits[corev1.ResourceMemory]
	if memLim.String() != "2Gi" {
		t.Errorf("Memory limit: got %s, want 2Gi", memLim.String())
	}

	// Verify requests match limits
	cpuReq := resources.Requests[corev1.ResourceCPU]
	if cpuReq.String() != "2" {
		t.Errorf("CPU request: got %s, want 2", cpuReq.String())
	}

	memReq := resources.Requests[corev1.ResourceMemory]
	if memReq.String() != "2Gi" {
		t.Errorf("Memory request: got %s, want 2Gi", memReq.String())
	}
}

func TestBuilder_Build_PartialResourceLimits(t *testing.T) {
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "partial-resources",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
				ResourceLimits: &werfv1alpha1.ResourceLimitsConfig{
					CPU: "500m",
					// Memory not specified, should use default
				},
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)
	job, err := builder.Build("v1.0.0")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := job.Spec.Template.Spec.Containers[0]
	resources := container.Resources

	// Verify custom CPU limit is used
	cpuLim := resources.Limits[corev1.ResourceCPU]
	if cpuLim.String() != "500m" {
		t.Errorf("CPU limit: got %s, want 500m", cpuLim.String())
	}

	// Verify default memory limit is used
	memLim := resources.Limits[corev1.ResourceMemory]
	if memLim.String() != defaultMemory {
		t.Errorf("Memory limit: got %s, want 1Gi (default)", memLim.String())
	}
}

func TestBuilder_Build_DefaultResourceLimits(t *testing.T) {
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-resources",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
				// ResourceLimits not specified, should use defaults
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)
	job, err := builder.Build("v1.0.0")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	container := job.Spec.Template.Spec.Containers[0]
	resources := container.Resources

	// Verify default limits are applied
	cpuLim := resources.Limits[corev1.ResourceCPU]
	if cpuLim.String() != "1" {
		t.Errorf("CPU limit: got %s, want 1 (default)", cpuLim.String())
	}

	memLim := resources.Limits[corev1.ResourceMemory]
	if memLim.String() != defaultMemory {
		t.Errorf("Memory limit: got %s, want 1Gi (default)", memLim.String())
	}
}

func TestBuilder_Build_CustomLogRetention(t *testing.T) {
	retentionDays := int32(14)
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-retention",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
				LogRetentionDays:   &retentionDays,
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)
	job, err := builder.Build("v1.0.0")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify TTL is set correctly: 14 days = 1209600 seconds
	expectedTTL := int32(14 * 24 * 60 * 60) // 1209600
	if job.Spec.TTLSecondsAfterFinished == nil {
		t.Fatal("expected TTL to be set")
	}
	if *job.Spec.TTLSecondsAfterFinished != expectedTTL {
		t.Errorf("TTL: got %d seconds, want %d seconds", *job.Spec.TTLSecondsAfterFinished, expectedTTL)
	}
}

func TestBuilder_Build_PartialLogRetention(t *testing.T) {
	retentionDays := int32(3)
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "partial-retention",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
				LogRetentionDays:   &retentionDays,
				// Other fields like ResourceLimits not specified, should use defaults
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)
	job, err := builder.Build("v1.0.0")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify TTL is set correctly: 3 days = 259200 seconds
	expectedTTL := int32(3 * 24 * 60 * 60) // 259200
	if job.Spec.TTLSecondsAfterFinished == nil {
		t.Fatal("expected TTL to be set")
	}
	if *job.Spec.TTLSecondsAfterFinished != expectedTTL {
		t.Errorf("TTL: got %d seconds, want %d seconds", *job.Spec.TTLSecondsAfterFinished, expectedTTL)
	}
}

func TestBuilder_Build_DefaultLogRetention(t *testing.T) {
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-retention",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
				// LogRetentionDays not specified, should use default 7 days
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)
	job, err := builder.Build("v1.0.0")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify TTL is set to default: 7 days = 604800 seconds
	expectedTTL := int32(7 * 24 * 60 * 60) // 604800
	if job.Spec.TTLSecondsAfterFinished == nil {
		t.Fatal("expected TTL to be set")
	}
	if *job.Spec.TTLSecondsAfterFinished != expectedTTL {
		t.Errorf("TTL: got %d seconds, want %d seconds (default 7 days)", *job.Spec.TTLSecondsAfterFinished, expectedTTL)
	}
}

func TestBuilder_Build_UniqueUUIDs(t *testing.T) {
	bundle := &werfv1alpha1.WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-uuid-uniqueness",
			Namespace: "default",
		},
		Spec: werfv1alpha1.WerfBundleSpec{
			Registry: werfv1alpha1.RegistryConfig{
				URL: "ghcr.io/test/bundle",
			},
			Converge: werfv1alpha1.ConvergeConfig{
				ServiceAccountName: "werf-converge",
			},
		},
	}

	builder := NewBuilder(bundle).WithScheme(testScheme)

	// Generate multiple jobs and verify each has a unique UUID
	jobNames := make(map[string]bool)
	uuids := make(map[string]bool)

	for i := 0; i < 10; i++ {
		job, err := builder.Build("v1.0.0")
		if err != nil {
			t.Fatalf("Build failed: %v", err)
		}

		if jobNames[job.Name] {
			t.Errorf("duplicate job name generated: %s", job.Name)
		}
		jobNames[job.Name] = true

		parts := splitJobName(job.Name)
		if uuids[parts.uuid] {
			t.Errorf("duplicate UUID generated: %s", parts.uuid)
		}
		uuids[parts.uuid] = true

		// Verify UUID is 8 hex characters
		if len(parts.uuid) != 8 {
			t.Errorf("UUID length: got %d, want 8", len(parts.uuid))
		}

		// Verify it's valid hex
		for _, ch := range parts.uuid {
			isDigit := ch >= '0' && ch <= '9'
			isLowerHex := ch >= 'a' && ch <= 'f'
			if !isDigit && !isLowerHex {
				t.Errorf("UUID contains non-hex character: %c", ch)
			}
		}
	}

	// Verify we generated 10 unique names
	if len(jobNames) != 10 {
		t.Errorf("expected 10 unique job names, got %d", len(jobNames))
	}

	// Verify we generated 10 unique UUIDs
	if len(uuids) != 10 {
		t.Errorf("expected 10 unique UUIDs, got %d", len(uuids))
	}
}
