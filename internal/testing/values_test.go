package testing

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateTestConfigMapWithValues_CreatesResource(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	values := map[string]string{
		"app.name":     "myapp",
		"app.replicas": "3",
	}

	cm, err := CreateTestConfigMapWithValues(ctx, k8sClient, "default", "test-config", values)
	if err != nil {
		t.Fatalf("failed to create ConfigMap: %v", err)
	}

	if cm == nil {
		t.Fatal("expected ConfigMap to be returned, got nil")
	}

	if cm.Name != "test-config" {
		t.Errorf("expected ConfigMap name 'test-config', got '%s'", cm.Name)
	}

	if cm.Namespace != "default" {
		t.Errorf("expected ConfigMap namespace 'default', got '%s'", cm.Namespace)
	}
}

func TestCreateTestConfigMapWithValues_HasCorrectData(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	values := map[string]string{
		"app.name":     "myapp",
		"app.replicas": "3",
	}

	cm, err := CreateTestConfigMapWithValues(ctx, k8sClient, "default", "test-config", values)
	if err != nil {
		t.Fatalf("failed to create ConfigMap: %v", err)
	}

	if cm == nil {
		t.Fatal("expected ConfigMap to be returned, got nil")
	}

	// Verify values.yaml key exists
	if _, ok := cm.Data["values.yaml"]; !ok {
		t.Fatal("expected 'values.yaml' key in ConfigMap data, not found")
	}

	// Verify we can get the data back
	retrievedCM := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-config", Namespace: "default"}, retrievedCM); err != nil {
		t.Errorf("failed to retrieve ConfigMap: %v", err)
	}

	if retrievedCM.Data["values.yaml"] == "" {
		t.Error("expected values.yaml to contain YAML data, got empty string")
	}
}

func TestCreateTestConfigMapWithValues_HandlesEmptyValues(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Empty values map
	values := make(map[string]string)

	cm, err := CreateTestConfigMapWithValues(ctx, k8sClient, "default", "empty-config", values)
	if err != nil {
		t.Fatalf("failed to create ConfigMap with empty values: %v", err)
	}

	if cm == nil {
		t.Fatal("expected ConfigMap to be returned, got nil")
	}

	if _, ok := cm.Data["values.yaml"]; !ok {
		t.Fatal("expected 'values.yaml' key even with empty values")
	}
}

func TestCreateTestConfigMapWithValues_HandlesSpecialChars(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	values := map[string]string{
		"config.url":      "https://example.com/path?query=value",
		"config.password": "p@ssw0rd!",
		"config.path":     "/home/user/file.txt",
		"config.json":     `{"key": "value"}`,
	}

	cm, err := CreateTestConfigMapWithValues(ctx, k8sClient, "default", "special-chars", values)
	if err != nil {
		t.Fatalf("failed to create ConfigMap with special characters: %v", err)
	}

	if cm == nil {
		t.Fatal("expected ConfigMap to be returned, got nil")
	}

	if _, ok := cm.Data["values.yaml"]; !ok {
		t.Fatal("expected 'values.yaml' key in ConfigMap data")
	}
}

func TestCreateTestSecretWithValues_CreatesResource(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	values := map[string]string{
		"db.username": "user",
		"db.password": "secret-pass",
	}

	secret, err := CreateTestSecretWithValues(ctx, k8sClient, "default", "test-secret", values)
	if err != nil {
		t.Fatalf("failed to create Secret: %v", err)
	}

	if secret == nil {
		t.Fatal("expected Secret to be returned, got nil")
	}

	if secret.Name != "test-secret" {
		t.Errorf("expected Secret name 'test-secret', got '%s'", secret.Name)
	}

	if secret.Namespace != "default" {
		t.Errorf("expected Secret namespace 'default', got '%s'", secret.Namespace)
	}
}

func TestCreateTestSecretWithValues_HasBase64Data(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	values := map[string]string{
		"db.username": "user",
		"db.password": "secret-pass",
	}

	secret, err := CreateTestSecretWithValues(ctx, k8sClient, "default", "test-secret", values)
	if err != nil {
		t.Fatalf("failed to create Secret: %v", err)
	}

	if secret == nil {
		t.Fatal("expected Secret to be returned, got nil")
	}

	// Verify values.yaml key exists in Data
	if _, ok := secret.Data["values.yaml"]; !ok {
		t.Fatal("expected 'values.yaml' key in Secret data, not found")
	}

	// Verify we can get the data back
	retrievedSecret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-secret", Namespace: "default"}, retrievedSecret); err != nil {
		t.Errorf("failed to retrieve Secret: %v", err)
	}

	if len(retrievedSecret.Data["values.yaml"]) == 0 {
		t.Error("expected values.yaml to contain YAML data, got empty")
	}
}

func TestCreateTestSecretWithValues_DataIsDecodable(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	values := map[string]string{
		"db.username": "testuser",
		"db.password": "testpass",
	}

	secret, err := CreateTestSecretWithValues(ctx, k8sClient, "default", "decode-test", values)
	if err != nil {
		t.Fatalf("failed to create Secret: %v", err)
	}

	if secret == nil {
		t.Fatal("expected Secret to be returned, got nil")
	}

	// Verify the data is valid YAML by checking it contains expected keys
	yamlData := string(secret.Data["values.yaml"])
	if yamlData == "" {
		t.Error("expected YAML data to be non-empty")
	}

	// Check that the YAML contains our original keys
	if !contains(yamlData, "db.username") {
		t.Error("expected YAML to contain 'db.username' key")
	}
	if !contains(yamlData, "db.password") {
		t.Error("expected YAML to contain 'db.password' key")
	}
}

func TestCreateTestSecretWithValues_HandlesEmptyValues(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Empty values map
	values := make(map[string]string)

	secret, err := CreateTestSecretWithValues(ctx, k8sClient, "default", "empty-secret", values)
	if err != nil {
		t.Fatalf("failed to create Secret with empty values: %v", err)
	}

	if secret == nil {
		t.Fatal("expected Secret to be returned, got nil")
	}

	if _, ok := secret.Data["values.yaml"]; !ok {
		t.Fatal("expected 'values.yaml' key even with empty values")
	}
}

// Helper function for string checking
func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExtractSetFlags_SimpleFlags(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "werf",
							Args: []string{
								"converge",
								"--set", "app.name=myapp",
								"--set", "app.replicas=3",
								"my-registry/bundle:tag",
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	flags := ExtractSetFlags(job)
	if len(flags) != 2 {
		t.Errorf("expected 2 flags, got %d", len(flags))
	}

	if flags["app.name"] != "myapp" {
		t.Errorf("expected app.name=myapp, got %s", flags["app.name"])
	}

	if flags["app.replicas"] != "3" {
		t.Errorf("expected app.replicas=3, got %s", flags["app.replicas"])
	}
}

func TestExtractSetFlags_MultipleFlags(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "werf",
							Args: []string{
								"converge",
								"--set", "app.name=myapp",
								"--set", "database.host=db.example.com",
								"--set", "database.port=5432",
								"--set", "cache.enabled=true",
								"my-registry/bundle:tag",
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	flags := ExtractSetFlags(job)
	if len(flags) != 4 {
		t.Errorf("expected 4 flags, got %d", len(flags))
	}

	expectedFlags := map[string]string{
		"app.name":       "myapp",
		"database.host":  "db.example.com",
		"database.port":  "5432",
		"cache.enabled":  "true",
	}

	for key, expectedValue := range expectedFlags {
		if flags[key] != expectedValue {
			t.Errorf("for key %s: expected %s, got %s", key, expectedValue, flags[key])
		}
	}
}

func TestExtractSetFlags_EscapedValues(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "werf",
							Args: []string{
								"converge",
								"--set", "config.url=https://example.com/path?query=value",
								"--set", "config.password=p@ssw0rd!",
								"--set", "config.path=/home/user/file.txt",
								"my-registry/bundle:tag",
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	flags := ExtractSetFlags(job)
	if len(flags) != 3 {
		t.Errorf("expected 3 flags, got %d", len(flags))
	}

	if flags["config.url"] != "https://example.com/path?query=value" {
		t.Errorf("expected URL to be preserved, got %s", flags["config.url"])
	}

	if flags["config.password"] != "p@ssw0rd!" {
		t.Errorf("expected password to be preserved, got %s", flags["config.password"])
	}
}

func TestExtractSetFlags_NoFlags(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "werf",
							Args: []string{
								"converge",
								"my-registry/bundle:tag",
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	flags := ExtractSetFlags(job)
	if len(flags) != 0 {
		t.Errorf("expected 0 flags when no --set present, got %d", len(flags))
	}
}

func TestExtractSetFlags_MalformedFlag(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "werf",
							Args: []string{
								"converge",
								"--set", "valid.key=value",
								"--set", "malformed",
								"--set", "another.key=value2",
								"my-registry/bundle:tag",
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	flags := ExtractSetFlags(job)
	// Should extract valid flags and skip malformed ones
	if len(flags) != 2 {
		t.Errorf("expected 2 valid flags (skipping malformed), got %d", len(flags))
	}

	if flags["valid.key"] != "value" {
		t.Errorf("expected valid.key=value, got %s", flags["valid.key"])
	}

	if flags["another.key"] != "value2" {
		t.Errorf("expected another.key=value2, got %s", flags["another.key"])
	}
}
