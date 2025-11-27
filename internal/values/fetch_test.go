package values

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFetchConfigMap(t *testing.T) {
	tests := []struct {
		name            string
		configMaps      []*corev1.ConfigMap
		cmName          string
		bundleNamespace string
		targetNamespace string
		wantData        map[string]string
		wantErr         bool
		errContains     string
	}{
		{
			name: "ConfigMap in bundle namespace",
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "bundle-ns",
					},
					Data: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
			},
			cmName:          "test-config",
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			wantData: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name: "ConfigMap in target namespace only",
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "target-ns",
					},
					Data: map[string]string{
						"key1": "target-value1",
					},
				},
			},
			cmName:          "test-config",
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			wantData: map[string]string{
				"key1": "target-value1",
			},
			wantErr: false,
		},
		{
			name: "ConfigMap in both namespaces - bundle wins",
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "bundle-ns",
					},
					Data: map[string]string{
						"key1": "bundle-value",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "target-ns",
					},
					Data: map[string]string{
						"key1": "target-value",
					},
				},
			},
			cmName:          "test-config",
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			wantData: map[string]string{
				"key1": "bundle-value",
			},
			wantErr: false,
		},
		{
			name:            "ConfigMap not found in either namespace",
			configMaps:      []*corev1.ConfigMap{},
			cmName:          "missing-config",
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			wantData:        nil,
			wantErr:         true,
			errContains:     "not found in namespaces",
		},
		{
			name:            "ConfigMap not found in single namespace",
			configMaps:      []*corev1.ConfigMap{},
			cmName:          "missing-config",
			bundleNamespace: "bundle-ns",
			targetNamespace: "bundle-ns", // Same namespace
			wantData:        nil,
			wantErr:         true,
			errContains:     "not found in namespace",
		},
		{
			name: "Empty target namespace falls back to bundle namespace only",
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "bundle-ns",
					},
					Data: map[string]string{
						"key1": "value1",
					},
				},
			},
			cmName:          "test-config",
			bundleNamespace: "bundle-ns",
			targetNamespace: "", // Empty target
			wantData: map[string]string{
				"key1": "value1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with test ConfigMaps
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			objs := make([]runtime.Object, len(tt.configMaps))
			for i, cm := range tt.configMaps {
				objs[i] = cm
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				Build()

			// Call fetchConfigMap
			ctx := context.Background()
			gotData, err := fetchConfigMap(ctx, fakeClient, tt.cmName, tt.bundleNamespace, tt.targetNamespace)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("fetchConfigMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("fetchConfigMap() error = %v, should contain %q", err, tt.errContains)
				}
				return
			}

			// Check data
			if !mapsEqual(gotData, tt.wantData) {
				t.Errorf("fetchConfigMap() data = %v, want %v", gotData, tt.wantData)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper function to compare maps
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
