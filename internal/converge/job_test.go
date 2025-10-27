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

var testScheme = func() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(werfv1alpha1.AddToScheme(scheme))
	return scheme
}()

func TestBuilder_Build_ValidBundle(t *testing.T) {
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
	if job.Labels["app.kubernetes.io/instance"] != "test-app" {
		t.Errorf("instance label: got %q, want %q", job.Labels["app.kubernetes.io/instance"], "test-app")
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

	// Build same job twice with same tag
	job1, _ := builder.Build("v1.0.0")
	job2, _ := builder.Build("v1.0.0")

	if job1.Name != job2.Name {
		t.Errorf("job names differ: %q vs %q", job1.Name, job2.Name)
	}

	// Build job with different tag
	job3, _ := builder.Build("v2.0.0")
	if job1.Name == job3.Name {
		t.Errorf("jobs with different tags should have different names")
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
	if cpuReq.String() != "100m" {
		t.Errorf("CPU request: got %s, want 100m", cpuReq.String())
	}

	memReq := resources.Requests[corev1.ResourceMemory]
	if memReq.String() != "256Mi" {
		t.Errorf("Memory request: got %s, want 256Mi", memReq.String())
	}

	// Verify limits
	if resources.Limits == nil {
		t.Fatal("resource limits are nil")
	}

	cpuLim := resources.Limits[corev1.ResourceCPU]
	if cpuLim.String() != "2" {
		t.Errorf("CPU limit: got %s, want 2", cpuLim.String())
	}

	memLim := resources.Limits[corev1.ResourceMemory]
	if memLim.String() != "2Gi" {
		t.Errorf("Memory limit: got %s, want 2Gi", memLim.String())
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
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 1 {
		t.Error("expected backoff limit of 1")
	}

	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != 3600 {
		t.Error("expected TTL of 3600 seconds")
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
