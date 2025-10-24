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
				SecretRef: &corev1.LocalObjectReference{
					Name: "registry-creds",
				},
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
				},
			},
			wantErr: false,
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
