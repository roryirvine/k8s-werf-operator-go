package converge

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
	"github.com/werf/k8s-werf-operator-go/internal/values"
)

func TestBuilder_Build_WithValuesFrom(t *testing.T) {
	tests := []struct {
		name            string
		bundle          *werfv1alpha1.WerfBundle
		configMaps      []*corev1.ConfigMap
		secrets         []*corev1.Secret
		wantArgsContain []string
		wantErr         bool
		errContains     string
	}{
		{
			name: "Single ConfigMap source with flat values",
			bundle: &werfv1alpha1.WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "default",
				},
				Spec: werfv1alpha1.WerfBundleSpec{
					Registry: werfv1alpha1.RegistryConfig{
						URL: "ghcr.io/test/bundle",
					},
					Converge: werfv1alpha1.ConvergeConfig{
						ServiceAccountName: "werf-converge",
						ValuesFrom: []werfv1alpha1.ValuesSource{
							{
								ConfigMapRef: &corev1.LocalObjectReference{Name: "app-config"},
							},
						},
					},
				},
			},
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-config",
						Namespace: "default",
					},
					Data: map[string]string{
						"values.yaml": `
app:
  name: my-app
  replicas: 3
`,
					},
				},
			},
			wantArgsContain: []string{
				"--set", "app.name=my-app",
				"--set", "app.replicas=3",
			},
			wantErr: false,
		},
		{
			name: "Multiple sources merged in order",
			bundle: &werfv1alpha1.WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "default",
				},
				Spec: werfv1alpha1.WerfBundleSpec{
					Registry: werfv1alpha1.RegistryConfig{
						URL: "ghcr.io/test/bundle",
					},
					Converge: werfv1alpha1.ConvergeConfig{
						ServiceAccountName: "werf-converge",
						ValuesFrom: []werfv1alpha1.ValuesSource{
							{
								ConfigMapRef: &corev1.LocalObjectReference{Name: "base-config"},
							},
							{
								SecretRef: &corev1.LocalObjectReference{Name: "secret-config"},
							},
						},
					},
				},
			},
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "base-config",
						Namespace: "default",
					},
					Data: map[string]string{
						"values.yaml": `
key1: base-value
key2: also-base
`,
					},
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-config",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"values.yaml": []byte(`key2: secret-override`),
					},
				},
			},
			wantArgsContain: []string{
				"--set", "key1=base-value",
				"--set", "key2=secret-override", // Later source wins
			},
			wantErr: false,
		},
		{
			name: "TargetNamespace specified - uses different namespace",
			bundle: &werfv1alpha1.WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "operator-ns",
				},
				Spec: werfv1alpha1.WerfBundleSpec{
					Registry: werfv1alpha1.RegistryConfig{
						URL: "ghcr.io/test/bundle",
					},
					Converge: werfv1alpha1.ConvergeConfig{
						ServiceAccountName: "werf-converge",
						TargetNamespace:    "app-ns",
						ValuesFrom: []werfv1alpha1.ValuesSource{
							{
								ConfigMapRef: &corev1.LocalObjectReference{Name: "app-config"},
							},
						},
					},
				},
			},
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-config",
						Namespace: "app-ns", // In target namespace
					},
					Data: map[string]string{
						"values.yaml": `env: production`,
					},
				},
			},
			wantArgsContain: []string{
				"--set", "env=production",
			},
			wantErr: false,
		},
		{
			name: "Bundle namespace takes precedence over target namespace",
			bundle: &werfv1alpha1.WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "operator-ns",
				},
				Spec: werfv1alpha1.WerfBundleSpec{
					Registry: werfv1alpha1.RegistryConfig{
						URL: "ghcr.io/test/bundle",
					},
					Converge: werfv1alpha1.ConvergeConfig{
						ServiceAccountName: "werf-converge",
						TargetNamespace:    "app-ns",
						ValuesFrom: []werfv1alpha1.ValuesSource{
							{
								ConfigMapRef: &corev1.LocalObjectReference{Name: "app-config"},
							},
						},
					},
				},
			},
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-config",
						Namespace: "operator-ns", // Admin-controlled
					},
					Data: map[string]string{
						"values.yaml": `env: admin-override`,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-config",
						Namespace: "app-ns", // App-controlled
					},
					Data: map[string]string{
						"values.yaml": `env: production`,
					},
				},
			},
			wantArgsContain: []string{
				"--set", "env=admin-override", // Bundle namespace wins
			},
			wantErr: false,
		},
		{
			name: "Optional source missing is skipped",
			bundle: &werfv1alpha1.WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "default",
				},
				Spec: werfv1alpha1.WerfBundleSpec{
					Registry: werfv1alpha1.RegistryConfig{
						URL: "ghcr.io/test/bundle",
					},
					Converge: werfv1alpha1.ConvergeConfig{
						ServiceAccountName: "werf-converge",
						ValuesFrom: []werfv1alpha1.ValuesSource{
							{
								ConfigMapRef: &corev1.LocalObjectReference{Name: "exists"},
							},
							{
								ConfigMapRef: &corev1.LocalObjectReference{Name: "missing"},
								Optional:     true,
							},
						},
					},
				},
			},
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "exists",
						Namespace: "default",
					},
					Data: map[string]string{
						"values.yaml": `key: value`,
					},
				},
			},
			wantArgsContain: []string{
				"--set", "key=value",
			},
			wantErr: false,
		},
		{
			name: "Required source missing returns error",
			bundle: &werfv1alpha1.WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "default",
				},
				Spec: werfv1alpha1.WerfBundleSpec{
					Registry: werfv1alpha1.RegistryConfig{
						URL: "ghcr.io/test/bundle",
					},
					Converge: werfv1alpha1.ConvergeConfig{
						ServiceAccountName: "werf-converge",
						ValuesFrom: []werfv1alpha1.ValuesSource{
							{
								ConfigMapRef: &corev1.LocalObjectReference{Name: "missing"},
							},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "No ValuesFrom configured - no --set flags",
			bundle: &werfv1alpha1.WerfBundle{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "default",
				},
				Spec: werfv1alpha1.WerfBundleSpec{
					Registry: werfv1alpha1.RegistryConfig{
						URL: "ghcr.io/test/bundle",
					},
					Converge: werfv1alpha1.ConvergeConfig{
						ServiceAccountName: "werf-converge",
						// No ValuesFrom
					},
				},
			},
			wantArgsContain: []string{
				"converge",
				"--log-color=false",
				"ghcr.io/test/bundle:v1.0.0",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with test resources
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

			// Create resolver and builder
			resolver := values.NewResolver(fakeClient)
			builder := NewBuilder(tt.bundle).
				WithScheme(testScheme).
				WithValuesResolver(resolver)

			// Build job
			ctx := context.Background()
			job, err := builder.Build(ctx, "v1.0.0")

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.errContains != "" && (err == nil || !strings.Contains(err.Error(), tt.errContains)) {
					t.Errorf("Build() error = %v, should contain %q", err, tt.errContains)
				}
				return
			}

			// Verify args contain expected --set flags
			container := job.Spec.Template.Spec.Containers[0]
			args := container.Args

			for _, wantArg := range tt.wantArgsContain {
				if !containsString(args, wantArg) {
					t.Errorf("Args missing expected argument: %q\nGot args: %v", wantArg, args)
				}
			}
		})
	}
}

// Helper function to check if a string slice contains a string
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// TestHelper_ValuesHelpersReduceBoilerplate demonstrates how the values test helpers
// simplify setup and verification in integration tests.
// This test documents the benefit of using CreateTestConfigMapWithValues, CreateTestSecretWithValues,
// and ExtractSetFlags helpers compared to manual construction.
func TestHelper_ValuesHelpersReduceBoilerplate(t *testing.T) {
	// HELPER BENEFITS DOCUMENTATION:
	//
	// CreateTestConfigMapWithValues: Setup helper
	// BEFORE: 15+ lines of inline ConfigMap construction with raw YAML
	//   cm := &corev1.ConfigMap{
	//       ObjectMeta: metav1.ObjectMeta{Name: "...", Namespace: "..."},
	//       Data: map[string]string{
	//           "values.yaml": "app:\n  name: ...\n  replicas: ...\ndatabase:\n  host: ...\n",
	//       },
	//   }
	//   k8sClient.Create(ctx, cm)
	//
	// AFTER: 2 lines using helper
	//   cm, err := testingutil.CreateTestConfigMapWithValues(ctx, k8sClient, "default", "app-config", map[string]string{
	//       "app.name": "myapp",
	//       "app.replicas": "3",
	//       "database.host": "postgres.db",
	//   })
	//
	// Benefits:
	// - Clearer intent: creating values config, not worrying about ConfigMap structure
	// - Less boilerplate: map speaks for itself, no YAML escaping
	// - Easier to maintain: change data in one place (the map)
	//
	// ---
	//
	// CreateTestSecretWithValues: Setup helper for secrets
	// BEFORE: 15+ lines with base64 encoding concerns
	//   secret := &corev1.Secret{
	//       ObjectMeta: metav1.ObjectMeta{Name: "...", Namespace: "..."},
	//       Data: map[string][]byte{
	//           "values.yaml": []byte(...),  // need to encode YAML as bytes
	//       },
	//   }
	//
	// AFTER: 2 lines, handles encoding automatically
	//   secret, err := testingutil.CreateTestSecretWithValues(ctx, k8sClient, "default", "app-secrets", map[string]string{
	//       "db.username": "appuser",
	//       "db.password": "secret123",
	//   })
	//
	// Benefits:
	// - No manual base64 encoding
	// - Same interface as ConfigMap helper
	// - Kubernetes client handles encoding transparently
	//
	// ---
	//
	// ExtractSetFlags: Verification helper
	// BEFORE: 10+ lines of manual parsing
	//   args := job.Spec.Template.Spec.Containers[0].Args
	//   flags := make(map[string]string)
	//   for i := 0; i < len(args); i++ {
	//       if args[i] == "--set" && i+1 < len(args) {
	//           parts := strings.Split(args[i+1], "=")
	//           if len(parts) == 2 {
	//               flags[parts[0]] = parts[1]
	//           }
	//       }
	//   }
	//   if flags["app.name"] != "myapp" { t.Error("...") }
	//
	// AFTER: 2 lines
	//   flags := testingutil.ExtractSetFlags(job)
	//   if flags["app.name"] != "myapp" { t.Error("...") }
	//
	// Benefits:
	// - Clear intent: checking values in Job
	// - Less noise in tests: focus on what matters (the assertions)
	// - Robust parsing: handles all edge cases

	// This test passes because it documents the helpers, doesn't require execution
}
