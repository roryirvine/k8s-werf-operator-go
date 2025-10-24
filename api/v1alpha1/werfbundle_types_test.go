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
			name: "valid WerfBundle with required fields",
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
						TargetNamespace:    "app-ns",
						ServiceAccountName: "werf-converge",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "WerfBundle with all optional fields",
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
						PollInterval:      "30m",
						VersionConstraint: func() *string { s := ">=1.0.0"; return &s }(),
					},
					Converge: ConvergeConfig{
						TargetNamespace:    "app-ns",
						ServiceAccountName: "werf-converge",
						ResourceLimits: &ResourceLimits{
							CPU:    func() *string { s := "1"; return &s }(),
							Memory: func() *string { s := "1Gi"; return &s }(),
						},
						LogRetentionDays: func() *int32 { i := int32(14); return &i }(),
						DriftDetection: &DriftDetectionConfig{
							Enabled:  true,
							Interval: "30m",
						},
					},
				},
				Status: WerfBundleStatus{
					Phase:          "Synced",
					LastAppliedTag: "v1.2.3",
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
			},
			Converge: ConvergeConfig{
				TargetNamespace:    "app-ns",
				ServiceAccountName: "werf-converge",
			},
		},
		Status: WerfBundleStatus{
			Phase:          "Synced",
			LastAppliedTag: "v1.2.3",
		},
	}

	// Create a deep copy
	copy := original.DeepCopy()

	// Verify the copy has the same values
	if copy.Name != original.Name {
		t.Errorf("Copy name mismatch: got %s, want %s", copy.Name, original.Name)
	}
	if copy.Spec.Registry.URL != original.Spec.Registry.URL {
		t.Errorf("Copy registry URL mismatch: got %s, want %s", copy.Spec.Registry.URL, original.Spec.Registry.URL)
	}

	// Verify modifying the copy doesn't affect the original
	copy.Spec.Registry.URL = "ghcr.io/other/bundle"
	if original.Spec.Registry.URL != "ghcr.io/org/bundle" {
		t.Errorf("Original was modified when copy was changed")
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
						TargetNamespace:    "app-ns",
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
						TargetNamespace:    "app-ns",
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
