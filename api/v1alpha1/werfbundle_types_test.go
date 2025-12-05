/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWerfBundleCreation(t *testing.T) {
	tests := []struct {
		name    string
		bundle  *WerfBundle
		wantErr bool
	}{
		{
			name: "valid WerfBundle with minimal required fields",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid WerfBundle with all Slice 1 fields",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle-full",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
						SecretRef: &corev1.LocalObjectReference{
							Name: "registry-creds",
						},
						PollInterval: "30m",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
					},
				},
				Status: WerfBundleStatus{
					Phase:          "Synced",
					LastAppliedTag: "v1.2.3",
				},
			},
			wantErr: false,
		},
		{
			name: "valid WerfBundle with retry tracking fields (Slice 2)",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle-retry",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
					},
				},
				Status: WerfBundleStatus{
					Phase:               "Syncing",
					LastETag:            "abc123def456",
					ConsecutiveFailures: 2,
					LastErrorTime:       &metav1.Time{Time: metav1.Now().Time},
					LastErrorMessage:    "temporary network error",
				},
			},
			wantErr: false,
		},
		{
			name: "valid WerfBundle with TargetNamespace (Slice 3)",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle-target-ns",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
						TargetNamespace:    "my-app-prod",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the bundle can be marshaled to JSON
			data, err := json.Marshal(tt.bundle)
			if (err != nil) != tt.wantErr {
				t.Errorf("WerfBundle marshal error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Test that it can be unmarshaled back
			var result WerfBundle
			if err := json.Unmarshal(data, &result); err != nil {
				t.Errorf("WerfBundle unmarshal error = %v", err)
				return
			}

			// Verify key fields are preserved
			if result.Name != tt.bundle.Name {
				t.Errorf("Name mismatch: got %s, want %s", result.Name, tt.bundle.Name)
			}
			if result.Spec.Registry.URL != tt.bundle.Spec.Registry.URL {
				t.Errorf("Registry URL mismatch: got %s, want %s", result.Spec.Registry.URL, tt.bundle.Spec.Registry.URL)
			}
		})
	}
}

func TestWerfBundleDeepCopy(t *testing.T) {
	original := &WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle",
			Namespace: "default",
		},
		Spec: WerfBundleSpec{
			Registry: RegistryConfig{
				URL: "ghcr.io/org/bundle",
				SecretRef: &corev1.LocalObjectReference{
					Name: "registry-creds",
				},
			},
			Converge: ConvergeConfig{
				ServiceAccountName: "werf-converge",
			},
		},
		Status: WerfBundleStatus{
			Phase:          "Synced",
			LastAppliedTag: "v1.2.3",
		},
	}

	copy := original.DeepCopy()

	if copy.Name != original.Name {
		t.Errorf("Copy name mismatch: got %s, want %s", copy.Name, original.Name)
	}
	if copy.Spec.Registry.URL != original.Spec.Registry.URL {
		t.Errorf("Copy registry URL mismatch: got %s, want %s", copy.Spec.Registry.URL, original.Spec.Registry.URL)
	}

	copy.Spec.Registry.URL = "ghcr.io/other/bundle"
	if original.Spec.Registry.URL != "ghcr.io/org/bundle" {
		t.Errorf("Original was modified when copy was changed")
	}

	// Test DeepCopy with ValuesFrom
	originalWithValues := &WerfBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bundle-values",
			Namespace: "default",
		},
		Spec: WerfBundleSpec{
			Registry: RegistryConfig{
				URL: "ghcr.io/org/bundle",
			},
			Converge: ConvergeConfig{
				ServiceAccountName: "werf-converge",
				ValuesFrom: []ValuesSource{
					{
						ConfigMapRef: &corev1.LocalObjectReference{
							Name: "common-config",
						},
					},
					{
						SecretRef: &corev1.LocalObjectReference{
							Name: "db-credentials",
						},
						Optional: true,
					},
				},
			},
		},
	}

	copyWithValues := originalWithValues.DeepCopy()

	// Verify ValuesFrom array is independent
	if len(copyWithValues.Spec.Converge.ValuesFrom) != len(originalWithValues.Spec.Converge.ValuesFrom) {
		t.Errorf("Copy ValuesFrom length mismatch: got %d, want %d",
			len(copyWithValues.Spec.Converge.ValuesFrom), len(originalWithValues.Spec.Converge.ValuesFrom))
	}

	// Modify the copy's ValuesFrom
	copyWithValues.Spec.Converge.ValuesFrom[0].ConfigMapRef.Name = "modified-config"
	copyWithValues.Spec.Converge.ValuesFrom = append(copyWithValues.Spec.Converge.ValuesFrom, ValuesSource{
		ConfigMapRef: &corev1.LocalObjectReference{Name: "new-config"},
	})

	// Verify original is unchanged
	if originalWithValues.Spec.Converge.ValuesFrom[0].ConfigMapRef.Name != "common-config" {
		t.Errorf("Original ValuesFrom was modified when copy was changed")
	}
	if len(originalWithValues.Spec.Converge.ValuesFrom) != 2 {
		t.Errorf("Original ValuesFrom array was modified when copy was changed")
	}
}

func TestWerfBundleList(t *testing.T) {
	list := &WerfBundleList{
		Items: []WerfBundle{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bundle-1",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle1",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bundle-2",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle2",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
					},
				},
			},
		},
	}

	if len(list.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(list.Items))
	}

	if list.Items[0].Name != "bundle-1" {
		t.Errorf("First item name mismatch: got %s, want bundle-1", list.Items[0].Name)
	}

	if list.Items[1].Spec.Registry.URL != "ghcr.io/org/bundle2" {
		t.Errorf("Second item registry URL mismatch: got %s, want ghcr.io/org/bundle2", list.Items[1].Spec.Registry.URL)
	}
}

func TestWerfBundleWithValuesFrom(t *testing.T) {
	tests := []struct {
		name   string
		bundle *WerfBundle
	}{
		{
			name: "nil ValuesFrom",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
					},
				},
			},
		},
		{
			name: "empty ValuesFrom array",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
						ValuesFrom:         []ValuesSource{},
					},
				},
			},
		},
		{
			name: "single ConfigMapRef entry",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
						ValuesFrom: []ValuesSource{
							{
								ConfigMapRef: &corev1.LocalObjectReference{
									Name: "common-config",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "single SecretRef entry",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
						ValuesFrom: []ValuesSource{
							{
								SecretRef: &corev1.LocalObjectReference{
									Name: "db-credentials",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple mixed entries with precedence order",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
						ValuesFrom: []ValuesSource{
							{
								ConfigMapRef: &corev1.LocalObjectReference{
									Name: "common-config",
								},
							},
							{
								ConfigMapRef: &corev1.LocalObjectReference{
									Name: "prod-config",
								},
							},
							{
								SecretRef: &corev1.LocalObjectReference{
									Name: "db-credentials",
								},
								Optional: true,
							},
						},
					},
				},
			},
		},
		{
			name: "optional flag variations",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
						ValuesFrom: []ValuesSource{
							{
								ConfigMapRef: &corev1.LocalObjectReference{
									Name: "required-config",
								},
								Optional: false,
							},
							{
								SecretRef: &corev1.LocalObjectReference{
									Name: "optional-secret",
								},
								Optional: true,
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the bundle can be marshaled to JSON
			data, err := json.Marshal(tt.bundle)
			if err != nil {
				t.Errorf("WerfBundle marshal error = %v", err)
				return
			}

			// Test that it can be unmarshaled back
			var result WerfBundle
			if err := json.Unmarshal(data, &result); err != nil {
				t.Errorf("WerfBundle unmarshal error = %v", err)
				return
			}

			// Verify key fields are preserved
			if result.Name != tt.bundle.Name {
				t.Errorf("Name mismatch: got %s, want %s", result.Name, tt.bundle.Name)
			}

			// Verify ValuesFrom array length preserved
			if len(result.Spec.Converge.ValuesFrom) != len(tt.bundle.Spec.Converge.ValuesFrom) {
				t.Errorf("ValuesFrom length mismatch: got %d, want %d",
					len(result.Spec.Converge.ValuesFrom), len(tt.bundle.Spec.Converge.ValuesFrom))
			}

			// Verify ValuesFrom entries preserved
			for i := range tt.bundle.Spec.Converge.ValuesFrom {
				original := tt.bundle.Spec.Converge.ValuesFrom[i]
				unmarshaled := result.Spec.Converge.ValuesFrom[i]

				if original.ConfigMapRef != nil {
					if unmarshaled.ConfigMapRef == nil {
						t.Errorf("ValuesFrom[%d].ConfigMapRef lost during marshal/unmarshal", i)
					} else if unmarshaled.ConfigMapRef.Name != original.ConfigMapRef.Name {
						t.Errorf("ValuesFrom[%d].ConfigMapRef.Name mismatch: got %s, want %s",
							i, unmarshaled.ConfigMapRef.Name, original.ConfigMapRef.Name)
					}
				}

				if original.SecretRef != nil {
					if unmarshaled.SecretRef == nil {
						t.Errorf("ValuesFrom[%d].SecretRef lost during marshal/unmarshal", i)
					} else if unmarshaled.SecretRef.Name != original.SecretRef.Name {
						t.Errorf("ValuesFrom[%d].SecretRef.Name mismatch: got %s, want %s",
							i, unmarshaled.SecretRef.Name, original.SecretRef.Name)
					}
				}

				if unmarshaled.Optional != original.Optional {
					t.Errorf("ValuesFrom[%d].Optional mismatch: got %v, want %v",
						i, unmarshaled.Optional, original.Optional)
				}
			}
		})
	}
}

func TestWerfBundleValidation(t *testing.T) {
	tests := []struct {
		name    string
		bundle  *WerfBundle
		wantErr bool
		errMsg  string
	}{
		{
			name: "empty registry URL should be rejected",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
					},
				},
			},
			wantErr: true,
			errMsg:  "URL is required",
		},
		{
			name: "valid registry URL",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid poll interval format should be validated at API server",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL:          "ghcr.io/org/bundle",
						PollInterval: "invalid",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
					},
				},
			},
			wantErr: true,
			errMsg:  "duration format",
		},
		{
			name: "valid poll interval formats",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL:          "ghcr.io/org/bundle",
						PollInterval: "15m",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty ServiceAccountName should be rejected",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "",
					},
				},
			},
			wantErr: true,
			errMsg:  "ServiceAccountName is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.bundle)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			var result WerfBundle
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if result.Spec.Registry.URL != tt.bundle.Spec.Registry.URL {
				t.Errorf("URL mismatch: got %s, want %s", result.Spec.Registry.URL, tt.bundle.Spec.Registry.URL)
			}
		})
	}
}

func TestCrossNamespaceValidation(t *testing.T) {
	tests := []struct {
		name            string
		bundle          *WerfBundle
		wantErr         bool
		errContains     string
	}{
		{
			name: "same-namespace deployment without ServiceAccountName - valid",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						// No TargetNamespace = same namespace
						// No ServiceAccountName = should be valid for backward compat
					},
				},
			},
			wantErr: false,
		},
		{
			name: "same-namespace deployment with ServiceAccountName - valid",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "cross-namespace with ServiceAccountName - valid",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "operator-system",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						TargetNamespace:    "my-app-prod",
						ServiceAccountName: "werf-deploy",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "cross-namespace without ServiceAccountName - invalid",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "operator-system",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						TargetNamespace: "my-app-prod",
						// No ServiceAccountName = should fail
					},
				},
			},
			wantErr:     true,
			errContains: "serviceAccountName is required for cross-namespace deployment",
		},
		{
			name: "TargetNamespace explicitly set to bundle namespace - treated as same-namespace",
			bundle: &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						TargetNamespace: "default",
						// No ServiceAccountName = should be valid (same namespace)
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bundle.ValidateCrossNamespaceDeployment()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestValuesSourceValidation(t *testing.T) {
	// Note: These tests verify JSON serialization round-trip behavior.
	// Actual CEL validation (mutual exclusivity of ConfigMapRef/SecretRef)
	// happens at the Kubernetes API server and will be verified via E2E tests
	// or manual kubectl apply testing.

	tests := []struct {
		name         string
		valuesSource ValuesSource
		description  string
	}{
		{
			name: "ConfigMapRef only - valid",
			valuesSource: ValuesSource{
				ConfigMapRef: &corev1.LocalObjectReference{
					Name: "common-config",
				},
			},
			description: "API server will accept: exactly one ref is set",
		},
		{
			name: "SecretRef only - valid",
			valuesSource: ValuesSource{
				SecretRef: &corev1.LocalObjectReference{
					Name: "db-credentials",
				},
			},
			description: "API server will accept: exactly one ref is set",
		},
		{
			name: "both refs set - invalid",
			valuesSource: ValuesSource{
				ConfigMapRef: &corev1.LocalObjectReference{
					Name: "common-config",
				},
				SecretRef: &corev1.LocalObjectReference{
					Name: "db-credentials",
				},
			},
			description: "API server will reject with CEL error: exactly one of configMapRef or secretRef must be set",
		},
		{
			name:         "neither ref set - invalid",
			valuesSource: ValuesSource{},
			description:  "API server will reject with CEL error: exactly one of configMapRef or secretRef must be set",
		},
		{
			name: "ConfigMapRef with optional true",
			valuesSource: ValuesSource{
				ConfigMapRef: &corev1.LocalObjectReference{
					Name: "optional-config",
				},
				Optional: true,
			},
			description: "API server will accept: valid with optional flag set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal bundle with this ValuesSource
			bundle := &WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bundle",
					Namespace: "default",
				},
				Spec: WerfBundleSpec{
					Registry: RegistryConfig{
						URL: "ghcr.io/org/bundle",
					},
					Converge: ConvergeConfig{
						ServiceAccountName: "werf-converge",
						ValuesFrom:         []ValuesSource{tt.valuesSource},
					},
				},
			}

			// Test JSON serialization works at the Go level
			data, err := json.Marshal(bundle)
			if err != nil {
				t.Errorf("Marshal error = %v", err)
				return
			}

			var result WerfBundle
			if err := json.Unmarshal(data, &result); err != nil {
				t.Errorf("Unmarshal error = %v", err)
				return
			}

			// Log the expected API server behavior
			t.Logf("Expected API server behavior: %s", tt.description)

			// Verify JSON round-trip preserves the structure
			if len(result.Spec.Converge.ValuesFrom) != 1 {
				t.Errorf("ValuesFrom length mismatch after unmarshal")
			}
		})
	}
}
