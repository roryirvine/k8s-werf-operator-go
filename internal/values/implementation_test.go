package values

import (
	"context"
	"testing"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolverImpl_ResolveValues(t *testing.T) {
	tests := []struct {
		name            string
		sources         []werfv1alpha1.ValuesSource
		configMaps      []*corev1.ConfigMap
		secrets         []*corev1.Secret
		bundleNamespace string
		targetNamespace string
		want            map[string]string
		wantErr         bool
		errContains     string
	}{
		{
			name:    "Empty sources returns empty map",
			sources: []werfv1alpha1.ValuesSource{},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name: "Single ConfigMap source",
			sources: []werfv1alpha1.ValuesSource{
				{
					ConfigMapRef: &corev1.LocalObjectReference{Name: "config1"},
				},
			},
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "config1",
						Namespace: "bundle-ns",
					},
					Data: map[string]string{
						"values.yaml": `key1: value1`,
					},
				},
			},
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			want: map[string]string{
				"key1": "value1",
			},
			wantErr: false,
		},
		{
			name: "Single Secret source",
			sources: []werfv1alpha1.ValuesSource{
				{
					SecretRef: &corev1.LocalObjectReference{Name: "secret1"},
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: "bundle-ns",
					},
					Data: map[string][]byte{
						"values.yaml": []byte(`key1: secret-value`),
					},
				},
			},
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			want: map[string]string{
				"key1": "secret-value",
			},
			wantErr: false,
		},
		{
			name: "Multiple sources are merged in order",
			sources: []werfv1alpha1.ValuesSource{
				{
					ConfigMapRef: &corev1.LocalObjectReference{Name: "config1"},
				},
				{
					ConfigMapRef: &corev1.LocalObjectReference{Name: "config2"},
				},
			},
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "config1",
						Namespace: "bundle-ns",
					},
					Data: map[string]string{
						"values.yaml": `
key1: value1
key2: value2
`,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "config2",
						Namespace: "bundle-ns",
					},
					Data: map[string]string{
						"values.yaml": `
key2: override2
key3: value3
`,
					},
				},
			},
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			want: map[string]string{
				"key1": "value1",
				"key2": "override2", // Later source wins
				"key3": "value3",
			},
			wantErr: false,
		},
		{
			name: "Mixed ConfigMap and Secret sources",
			sources: []werfv1alpha1.ValuesSource{
				{
					ConfigMapRef: &corev1.LocalObjectReference{Name: "config1"},
				},
				{
					SecretRef: &corev1.LocalObjectReference{Name: "secret1"},
				},
			},
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "config1",
						Namespace: "bundle-ns",
					},
					Data: map[string]string{
						"values.yaml": `key1: from-config`,
					},
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: "bundle-ns",
					},
					Data: map[string][]byte{
						"values.yaml": []byte(`key2: from-secret`),
					},
				},
			},
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			want: map[string]string{
				"key1": "from-config",
				"key2": "from-secret",
			},
			wantErr: false,
		},
		{
			name: "Required source not found returns error",
			sources: []werfv1alpha1.ValuesSource{
				{
					ConfigMapRef: &corev1.LocalObjectReference{Name: "missing"},
				},
			},
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			want:            nil,
			wantErr:         true,
			errContains:     "not found",
		},
		{
			name: "Optional source not found is skipped",
			sources: []werfv1alpha1.ValuesSource{
				{
					ConfigMapRef: &corev1.LocalObjectReference{Name: "exists"},
				},
				{
					ConfigMapRef: &corev1.LocalObjectReference{Name: "missing"},
					Optional:     true,
				},
				{
					ConfigMapRef: &corev1.LocalObjectReference{Name: "also-exists"},
				},
			},
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "exists",
						Namespace: "bundle-ns",
					},
					Data: map[string]string{
						"values.yaml": `key1: value1`,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "also-exists",
						Namespace: "bundle-ns",
					},
					Data: map[string]string{
						"values.yaml": `key2: value2`,
					},
				},
			},
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name: "Empty ConfigMapRef name returns error",
			sources: []werfv1alpha1.ValuesSource{
				{
					ConfigMapRef: &corev1.LocalObjectReference{Name: ""},
				},
			},
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			want:            nil,
			wantErr:         true,
			errContains:     "name is empty",
		},
		{
			name: "Empty SecretRef name returns error",
			sources: []werfv1alpha1.ValuesSource{
				{
					SecretRef: &corev1.LocalObjectReference{Name: ""},
				},
			},
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			want:            nil,
			wantErr:         true,
			errContains:     "name is empty",
		},
		{
			name: "Source from target namespace",
			sources: []werfv1alpha1.ValuesSource{
				{
					ConfigMapRef: &corev1.LocalObjectReference{Name: "config1"},
				},
			},
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "config1",
						Namespace: "target-ns",
					},
					Data: map[string]string{
						"values.yaml": `key1: from-target`,
					},
				},
			},
			bundleNamespace: "bundle-ns",
			targetNamespace: "target-ns",
			want: map[string]string{
				"key1": "from-target",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = werfv1alpha1.AddToScheme(scheme)

			objs := make([]runtime.Object, 0, len(tt.configMaps)+len(tt.secrets))
			for _, cm := range tt.configMaps {
				objs = append(objs, cm)
			}
			for _, secret := range tt.secrets {
				objs = append(objs, secret)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				Build()

			resolver := NewResolver(fakeClient)

			// Call ResolveValues
			ctx := context.Background()
			got, err := resolver.ResolveValues(
				ctx,
				tt.sources,
				tt.bundleNamespace,
				tt.targetNamespace,
			)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("ResolveValues() error = %v, should contain %q", err, tt.errContains)
				}
				return
			}

			// Check result
			if !mapsEqual(got, tt.want) {
				t.Errorf("ResolveValues() = %v, want %v", got, tt.want)
			}
		})
	}
}
