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
	"strings"

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
// The helper handles:
// - Converting Go map to YAML format
// - Creating ConfigMap with correct metadata
// - Persisting to Kubernetes API (via client.Create)
// - Returning created ConfigMap for cleanup and assertions
//
// Use this instead of manually constructing ConfigMaps with inline YAML:
//   BEFORE (15+ lines):
//     cm := &corev1.ConfigMap{
//         ObjectMeta: metav1.ObjectMeta{...},
//         Data: map[string]string{
//             "values.yaml": "app:\n  name: myapp\n  replicas: \"3\"\n",
//         },
//     }
//     k8sClient.Create(ctx, cm)
//
//   AFTER (2 lines):
//     cm, err := CreateTestConfigMapWithValues(ctx, k8sClient, "default", "my-config", map[string]string{
//         "app.name": "myapp",
//         "app.replicas": "3",
//     })
//
// The returned ConfigMap can be used for cleanup:
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
// The helper handles:
// - Converting Go map to YAML format
// - Creating Secret with correct metadata
// - Converting YAML string to bytes (Kubernetes handles base64 transparently)
// - Persisting to Kubernetes API (via client.Create)
// - Returning created Secret for cleanup and assertions
//
// Use this instead of manually constructing Secrets with inline YAML and encoding:
//   BEFORE (15+ lines with encoding complexity):
//     secret := &corev1.Secret{
//         ObjectMeta: metav1.ObjectMeta{...},
//         Data: map[string][]byte{
//             "values.yaml": []byte("db:\n  username: user\n  password: secret-pass\n"),
//         },
//     }
//     k8sClient.Create(ctx, secret)
//
//   AFTER (2 lines):
//     secret, err := CreateTestSecretWithValues(ctx, k8sClient, "default", "my-secret", map[string]string{
//         "db.username": "user",
//         "db.password": "secret-pass",
//     })
//
// The returned Secret can be used for cleanup:
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
	yamlData, err := convertMapToYAML(values)
	if err != nil {
		return nil, fmt.Errorf("failed to convert values to YAML: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"values.yaml": []byte(yamlData),
		},
	}

	if err := c.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("failed to create Secret: %w", err)
	}

	return secret, nil
}

// ExtractSetFlags extracts --set flags from a Job's container arguments.
// Returns a map of flag key-value pairs parsed from the Job's args.
// This is useful for verifying that values were correctly converted to werf CLI flags.
//
// The helper handles:
// - Safely accessing Job container args
// - Parsing --set key=value flag pairs
// - Handling values with special characters (URLs, paths, passwords)
// - Skipping malformed flags gracefully
// - Returning convenient map[string]string for assertions
//
// Use this instead of manually parsing Job args:
//   BEFORE (10+ lines of parsing):
//     args := job.Spec.Template.Spec.Containers[0].Args
//     flags := make(map[string]string)
//     for i := 0; i < len(args); i++ {
//         if args[i] == "--set" && i+1 < len(args) {
//             parts := strings.Split(args[i+1], "=")
//             if len(parts) == 2 {
//                 flags[parts[0]] = parts[1]
//             }
//         }
//     }
//     if flags["app.name"] != "myapp" { t.Error(...) }
//
//   AFTER (2 lines):
//     flags := ExtractSetFlags(job)
//     if flags["app.name"] != "myapp" { t.Error(...) }
//
// Example with single assertion:
//
//	flags := ExtractSetFlags(job)
//	if flags["app.name"] != "myapp" {
//	    t.Errorf("expected app.name=myapp, got %v", flags["app.name"])
//	}
//
// Example with multiple assertions:
//
//	flags := ExtractSetFlags(job)
//	expectedFlags := map[string]string{
//	    "app.name": "myapp",
//	    "app.replicas": "3",
//	    "database.host": "postgres.db",
//	}
//	for key, expectedValue := range expectedFlags {
//	    if flags[key] != expectedValue {
//	        t.Errorf("for key %s: expected %s, got %s", key, expectedValue, flags[key])
//	    }
//	}
func ExtractSetFlags(job *batchv1.Job) map[string]string {
	result := make(map[string]string)

	if job == nil || len(job.Spec.Template.Spec.Containers) == 0 {
		return result
	}

	args := job.Spec.Template.Spec.Containers[0].Args

	for i := 0; i < len(args); i++ {
		if args[i] == "--set" && i+1 < len(args) {
			// Parse key=value from next arg
			parts := strings.SplitN(args[i+1], "=", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
			i++ // Skip the value arg
		}
	}

	return result
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
