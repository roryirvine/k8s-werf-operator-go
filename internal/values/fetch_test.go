package values

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/werf/k8s-werf-operator-go/internal/values/testdata"
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
						"values.yaml": `
key1: value1
key2: value2
`,
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
						"values.yaml": `key1: target-value1`,
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
						"values.yaml": `key1: bundle-value`,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "target-ns",
					},
					Data: map[string]string{
						"values.yaml": `key1: target-value`,
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
						"values.yaml": `key1: value1`,
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
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
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

// TestFetchConfigMapWithFixtures demonstrates using test fixtures instead of inline data.
// This test shows how to load ConfigMap fixtures and use them in tests.
func TestFetchConfigMapWithFixtures(t *testing.T) {
	tests := []struct {
		name            string
		fixture         string
		bundleNamespace string
		targetNamespace string
		cmName          string
		wantErr         bool
	}{
		{
			name:            "fetch simple fixture in bundle namespace",
			fixture:         "simple-values.yaml",
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			cmName:          "simple-values",
			wantErr:         false,
		},
		{
			name:            "fetch nested fixture in bundle namespace",
			fixture:         "nested-values.yaml",
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			cmName:          "nested-values",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load fixture - this is cleaner than constructing inline
			cm, err := testdata.LoadConfigMapFixture(tt.fixture)
			if err != nil {
				t.Fatalf("failed to load fixture: %v", err)
			}

			// Assign to bundle namespace for testing
			cm.Namespace = tt.bundleNamespace

			// Create fake client with fixture
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cm).
				Build()

			// Test fetchConfigMap with fixture data
			ctx := context.Background()
			gotData, err := fetchConfigMap(ctx, fakeClient, tt.cmName, tt.bundleNamespace, tt.targetNamespace)

			// Verify error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("fetchConfigMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify we got data (not empty)
			if !tt.wantErr && len(gotData) == 0 {
				t.Error("fetchConfigMap() returned empty data")
			}
		})
	}
}

func TestFetchSecret(t *testing.T) {
	tests := []struct {
		name            string
		secrets         []*corev1.Secret
		secretName      string
		bundleNamespace string
		targetNamespace string
		wantData        map[string]string
		wantErr         bool
		errContains     string
	}{
		{
			name: "Secret in bundle namespace",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "bundle-ns",
					},
					Data: map[string][]byte{
						"values.yaml": []byte(`
key1: secret-value1
key2: secret-value2
`),
					},
				},
			},
			secretName:      "test-secret",
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			wantData: map[string]string{
				"key1": "secret-value1",
				"key2": "secret-value2",
			},
			wantErr: false,
		},
		{
			name: "Secret in target namespace only",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "target-ns",
					},
					Data: map[string][]byte{
						"values.yaml": []byte(`key1: target-secret-value1`),
					},
				},
			},
			secretName:      "test-secret",
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			wantData: map[string]string{
				"key1": "target-secret-value1",
			},
			wantErr: false,
		},
		{
			name: "Secret in both namespaces - bundle wins",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "bundle-ns",
					},
					Data: map[string][]byte{
						"values.yaml": []byte(`key1: bundle-secret`),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "target-ns",
					},
					Data: map[string][]byte{
						"values.yaml": []byte(`key1: target-secret`),
					},
				},
			},
			secretName:      "test-secret",
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			wantData: map[string]string{
				"key1": "bundle-secret",
			},
			wantErr: false,
		},
		{
			name:            "Secret not found in either namespace",
			secrets:         []*corev1.Secret{},
			secretName:      "missing-secret",
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			wantData:        nil,
			wantErr:         true,
			errContains:     "not found in namespaces",
		},
		{
			name:            "Secret not found in single namespace",
			secrets:         []*corev1.Secret{},
			secretName:      "missing-secret",
			bundleNamespace: "bundle-ns",
			targetNamespace: "bundle-ns",
			wantData:        nil,
			wantErr:         true,
			errContains:     "not found in namespace",
		},
		{
			name: "Empty target namespace falls back to bundle namespace only",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "bundle-ns",
					},
					Data: map[string][]byte{
						"values.yaml": []byte(`key1: value1`),
					},
				},
			},
			secretName:      "test-secret",
			bundleNamespace: "bundle-ns",
			targetNamespace: "",
			wantData: map[string]string{
				"key1": "value1",
			},
			wantErr: false,
		},
		{
			name: "Secret with YAML in multiple keys are merged",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "bundle-ns",
					},
					Data: map[string][]byte{
						"config1.yaml": []byte(`key1: value1`),
						"config2.yaml": []byte(`key2: value2`),
					},
				},
			},
			secretName:      "test-secret",
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			wantData: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with test Secrets
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)

			objs := make([]runtime.Object, len(tt.secrets))
			for i, secret := range tt.secrets {
				objs[i] = secret
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				Build()

			// Call fetchSecret
			ctx := context.Background()
			gotData, err := fetchSecret(
				ctx,
				fakeClient,
				tt.secretName,
				tt.bundleNamespace,
				tt.targetNamespace,
			)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("fetchSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("fetchSecret() error = %v, should contain %q", err, tt.errContains)
				}
				return
			}

			// Check data
			if !mapsEqual(gotData, tt.wantData) {
				t.Errorf("fetchSecret() data = %v, want %v", gotData, tt.wantData)
			}
		})
	}
}

func TestSecretDataToStringMap(t *testing.T) {
	tests := []struct {
		name string
		data map[string][]byte
		want map[string]string
	}{
		{
			name: "Empty data",
			data: map[string][]byte{},
			want: map[string]string{},
		},
		{
			name: "Simple string data",
			data: map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2"),
			},
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "Binary data",
			data: map[string][]byte{
				"binary": {0x48, 0x65, 0x6c, 0x6c, 0x6f},
			},
			want: map[string]string{
				"binary": "Hello",
			},
		},
		{
			name: "Empty string value",
			data: map[string][]byte{
				"empty": []byte(""),
			},
			want: map[string]string{
				"empty": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := secretDataToStringMap(tt.data)
			if !mapsEqual(got, tt.want) {
				t.Errorf("secretDataToStringMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
