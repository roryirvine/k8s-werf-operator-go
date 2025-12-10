// Package testdata provides helper functions for loading test fixtures.
package testdata

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// LoadConfigMapFixture loads a ConfigMap fixture from testdata/configmaps/.
// The filename should be the name of the YAML file (e.g., "simple-values.yaml").
// Returns a pointer to the loaded ConfigMap or an error if the file is not found or invalid.
func LoadConfigMapFixture(filename string) (*corev1.ConfigMap, error) {
	filePath := filepath.Join(fixtureDir(), "configmaps", filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ConfigMap fixture %q: %w", filename, err)
	}

	var cm corev1.ConfigMap
	if err := yaml.Unmarshal(data, &cm); err != nil {
		return nil, fmt.Errorf("failed to parse ConfigMap fixture %q: %w", filename, err)
	}

	return &cm, nil
}

// LoadSecretFixture loads a Secret fixture from testdata/secrets/.
// The filename should be the name of the YAML file (e.g., "database-credentials.yaml").
// Returns a pointer to the loaded Secret or an error if the file is not found or invalid.
func LoadSecretFixture(filename string) (*corev1.Secret, error) {
	filePath := filepath.Join(fixtureDir(), "secrets", filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Secret fixture %q: %w", filename, err)
	}

	var secret corev1.Secret
	if err := yaml.Unmarshal(data, &secret); err != nil {
		return nil, fmt.Errorf("failed to parse Secret fixture %q: %w", filename, err)
	}

	return &secret, nil
}

// WithNamespace sets the namespace on a Kubernetes object and returns it.
// This is a convenience helper for tests that need to assign fixtures to specific namespaces.
// It simplifies test setup by allowing assignment and namespace setting in one step.
// Usage: cm := testdata.LoadConfigMapFixture("simple-values.yaml")
//        cm.Namespace = "test-ns"
// Or: cm, _ := testdata.LoadConfigMapFixture("simple-values.yaml")
//     WithNamespace(cm, "test-ns")
func WithNamespace(obj interface{}, namespace string) interface{} {
	switch v := obj.(type) {
	case *corev1.ConfigMap:
		if v != nil {
			v.Namespace = namespace
		}
	case *corev1.Secret:
		if v != nil {
			v.Namespace = namespace
		}
	}
	return obj
}

// ConfigMapWithNamespace is a convenience helper that returns a ConfigMap with namespace set.
// This is more idiomatic Go than the generic WithNamespace function.
// Usage: cm := testdata.ConfigMapWithNamespace(testdata.LoadConfigMapFixture("simple-values.yaml"), "test-ns")
func ConfigMapWithNamespace(cm *corev1.ConfigMap, namespace string) *corev1.ConfigMap {
	if cm != nil {
		cm.Namespace = namespace
	}
	return cm
}

// SecretWithNamespace is a convenience helper that returns a Secret with namespace set.
// This is more idiomatic Go than the generic WithNamespace function.
// Usage: s := testdata.SecretWithNamespace(testdata.LoadSecretFixture("database-credentials.yaml"), "test-ns")
func SecretWithNamespace(s *corev1.Secret, namespace string) *corev1.Secret {
	if s != nil {
		s.Namespace = namespace
	}
	return s
}

// fixtureDir returns the path to the testdata directory.
// It locates the directory relative to this file's location using runtime.Caller.
func fixtureDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		// Fallback if runtime.Caller fails
		return "internal/values/testdata"
	}
	// This file is helpers.go in internal/values/testdata/
	// filepath.Dir returns internal/values/testdata
	return filepath.Dir(file)
}
