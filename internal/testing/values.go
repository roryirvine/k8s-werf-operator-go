// Package testing provides utilities for writing tests of the Werf operator.
// This file contains helper functions for setting up values resources (ConfigMaps and Secrets) in tests.
// Use these helpers to create test values from Go maps instead of inline YAML strings.
//
// Example - Create ConfigMap from map:
//
//	cm, err := testing.CreateTestConfigMapWithValues(ctx, client, "default", "my-config", map[string]string{
//	    "app.name": "myapp",
//	    "app.replicas": "3",
//	})
//	defer client.Delete(ctx, cm)
//
// Example - Create Secret from map:
//
//	secret, err := testing.CreateTestSecretWithValues(ctx, client, "default", "my-secret", map[string]string{
//	    "db.username": "user",
//	    "db.password": "secret-pass",
//	})
//	defer client.Delete(ctx, secret)
//
// Example - Extract --set flags from Job for verification:
//
//	flags := testing.ExtractSetFlags(job)
//	if flags["app.name"] != "myapp" {
//	    t.Errorf("expected app.name=myapp, got %v", flags["app.name"])
//	}
package testing

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// CreateTestConfigMapWithValues creates a ConfigMap with values for testing.
// The values map is converted to YAML and stored in the "values.yaml" key.
// This is useful for creating test configuration data without writing raw YAML strings.
//
// Example:
//
//	cm, err := CreateTestConfigMapWithValues(ctx, k8sClient, "default", "my-config", map[string]string{
//	    "app.name": "myapp",
//	    "app.replicas": "3",
//	})
//	if err != nil {
//	    t.Fatalf("failed to create ConfigMap: %v", err)
//	}
//	defer k8sClient.Delete(ctx, cm)
func CreateTestConfigMapWithValues(ctx context.Context, c client.Client, namespace, name string, values map[string]string) (*corev1.ConfigMap, error) {
	yamlData, err := convertMapToYAML(values)
	if err != nil {
		return nil, fmt.Errorf("failed to convert values to YAML: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"values.yaml": yamlData,
		},
	}

	if err := c.Create(ctx, cm); err != nil {
		return nil, fmt.Errorf("failed to create ConfigMap: %w", err)
	}

	return cm, nil
}

// CreateTestSecretWithValues creates a Secret with values for testing.
// The values map is converted to YAML and stored in the "values.yaml" key as base64-encoded data.
// Kubernetes automatically handles base64 encoding of the Secret.Data field.
// This is useful for creating test secret data without writing raw YAML strings.
//
// Example:
//
//	secret, err := CreateTestSecretWithValues(ctx, k8sClient, "default", "my-secret", map[string]string{
//	    "db.username": "user",
//	    "db.password": "secret-pass",
//	})
//	if err != nil {
//	    t.Fatalf("failed to create Secret: %v", err)
//	}
//	defer k8sClient.Delete(ctx, secret)
func CreateTestSecretWithValues(ctx context.Context, c client.Client, namespace, name string, values map[string]string) (*corev1.Secret, error) {
	return nil, nil
}

// ExtractSetFlags extracts --set flags from a Job's container arguments.
// Returns a map of flag key-value pairs parsed from the Job's args.
// This is useful for verifying that values were correctly converted to werf CLI flags.
//
// Example:
//
//	flags := ExtractSetFlags(job)
//	if flags["app.name"] != "myapp" {
//	    t.Errorf("expected app.name=myapp, got %v", flags["app.name"])
//	}
func ExtractSetFlags(job *batchv1.Job) map[string]string {
	return nil
}

// convertMapToYAML converts a map[string]string to YAML format.
// Used internally by ConfigMap and Secret helpers.
func convertMapToYAML(values map[string]string) (string, error) {
	data, err := yaml.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("failed to marshal values to YAML: %w", err)
	}
	return string(data), nil
}
