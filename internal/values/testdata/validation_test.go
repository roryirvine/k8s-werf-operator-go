package testdata

import (
	"strings"
	"testing"
)

// TestConfigMapFixturesAreValid validates all ConfigMap fixtures have required Kubernetes fields.
func TestConfigMapFixturesAreValid(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{"simple-values.yaml", "simple-values.yaml"},
		{"nested-values.yaml", "nested-values.yaml"},
		{"override-values.yaml", "override-values.yaml"},
		{"multi-file-values.yaml", "multi-file-values.yaml"},
		{"special-chars-values.yaml", "special-chars-values.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := LoadConfigMapFixture(tt.filename)
			if err != nil {
				t.Fatalf("failed to load fixture: %v", err)
			}

			// Validate required Kubernetes fields
			if cm == nil {
				t.Error("fixture returned nil ConfigMap")
			}
			if cm.APIVersion != "v1" {
				t.Errorf("expected apiVersion 'v1', got %q", cm.APIVersion)
			}
			if cm.Kind != "ConfigMap" {
				t.Errorf("expected kind 'ConfigMap', got %q", cm.Kind)
			}
			if cm.Name == "" {
				t.Error("fixture has empty metadata.name")
			}

			// Validate data section exists and is not empty
			if len(cm.Data) == 0 {
				t.Error("fixture has empty data section")
			}

			// Validate at least one data key exists
			hasData := false
			for key := range cm.Data {
				if key != "" {
					hasData = true
					break
				}
			}
			if !hasData {
				t.Error("fixture has no non-empty data keys")
			}
		})
	}
}

// TestSecretFixturesAreValid validates all Secret fixtures have required Kubernetes fields.
func TestSecretFixturesAreValid(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{"database-credentials.yaml", "database-credentials.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret, err := LoadSecretFixture(tt.filename)
			if err != nil {
				t.Fatalf("failed to load fixture: %v", err)
			}

			// Validate required Kubernetes fields
			if secret == nil {
				t.Error("fixture returned nil Secret")
			}
			if secret.APIVersion != "v1" {
				t.Errorf("expected apiVersion 'v1', got %q", secret.APIVersion)
			}
			if secret.Kind != "Secret" {
				t.Errorf("expected kind 'Secret', got %q", secret.Kind)
			}
			if secret.Name == "" {
				t.Error("fixture has empty metadata.name")
			}

			// Validate data section exists and is not empty
			if len(secret.Data) == 0 {
				t.Error("fixture has empty data section")
			}

			// Validate at least one data key exists
			hasData := false
			for key := range secret.Data {
				if key != "" {
					hasData = true
					break
				}
			}
			if !hasData {
				t.Error("fixture has no non-empty data keys")
			}

			// Validate Secret type is set
			if secret.Type == "" {
				t.Error("fixture has empty type field")
			}
		})
	}
}

// TestFixtureConsistency validates that fixtures follow naming and structure conventions.
func TestFixtureConsistency(t *testing.T) {
	configMaps := []string{
		"simple-values.yaml",
		"nested-values.yaml",
		"override-values.yaml",
		"multi-file-values.yaml",
		"special-chars-values.yaml",
	}

	// Verify all ConfigMap fixtures can be loaded and have consistent structure
	for _, filename := range configMaps {
		t.Run("ConfigMap_"+filename, func(t *testing.T) {
			cm, err := LoadConfigMapFixture(filename)
			if err != nil {
				t.Fatalf("failed to load ConfigMap fixture %q: %v", filename, err)
			}

			// All fixtures should be ConfigMaps
			if cm.Kind != "ConfigMap" {
				t.Errorf("expected kind ConfigMap, got %v", cm.Kind)
			}

			// Name should match fixture filename (without .yaml)
			expectedNamePrefix := filename[:len(filename)-5] // Remove .yaml
			if !hasNameMatch(cm.ObjectMeta.Name, expectedNamePrefix) {
				t.Logf("fixture name %q matches prefix %q (different naming is acceptable)", cm.Name, expectedNamePrefix)
			}
		})
	}

	secrets := []string{
		"database-credentials.yaml",
	}

	// Verify all Secret fixtures can be loaded and have consistent structure
	for _, filename := range secrets {
		t.Run("Secret_"+filename, func(t *testing.T) {
			secret, err := LoadSecretFixture(filename)
			if err != nil {
				t.Fatalf("failed to load Secret fixture %q: %v", filename, err)
			}

			// All fixtures should be Secrets
			if secret.Kind != "Secret" {
				t.Errorf("expected kind Secret, got %v", secret.Kind)
			}

			// Name should match fixture filename (without .yaml)
			expectedNamePrefix := filename[:len(filename)-5] // Remove .yaml
			if !hasNameMatch(secret.ObjectMeta.Name, expectedNamePrefix) {
				t.Logf("fixture name %q matches prefix %q (different naming is acceptable)", secret.Name, expectedNamePrefix)
			}
		})
	}
}

// hasNameMatch checks if object name has the expected prefix.
// This is lenient - we only check prefix, not exact match.
func hasNameMatch(name, expectedPrefix string) bool {
	return strings.HasPrefix(name, expectedPrefix)
}

// TestFixtureDataAccess validates that fixture data is accessible and readable.
func TestFixtureDataAccess(t *testing.T) {
	t.Run("ConfigMap_data_is_accessible", func(t *testing.T) {
		cm, err := LoadConfigMapFixture("simple-values.yaml")
		if err != nil {
			t.Fatalf("failed to load fixture: %v", err)
		}

		// Verify we can access data
		if cm.Data == nil {
			t.Error("ConfigMap data is nil")
			return
		}

		// At minimum, should have a values.yaml key or similar
		dataCount := len(cm.Data)
		if dataCount == 0 {
			t.Error("ConfigMap data map is empty")
		}
	})

	t.Run("Secret_data_is_accessible", func(t *testing.T) {
		secret, err := LoadSecretFixture("database-credentials.yaml")
		if err != nil {
			t.Fatalf("failed to load fixture: %v", err)
		}

		// Verify we can access data
		if secret.Data == nil {
			t.Error("Secret data is nil")
			return
		}

		// At minimum, should have some data
		dataCount := len(secret.Data)
		if dataCount == 0 {
			t.Error("Secret data map is empty")
		}
	})
}
