package testdata

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLoadConfigMapFixture(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
		validate func(t *testing.T, cm *corev1.ConfigMap)
	}{
		{
			name:     "load simple-values fixture",
			filename: "simple-values.yaml",
			wantErr:  false,
			validate: func(t *testing.T, cm *corev1.ConfigMap) {
				if cm.Name != "simple-values" {
					t.Errorf("expected name 'simple-values', got %q", cm.Name)
				}
				if cm.Kind != "ConfigMap" {
					t.Errorf("expected kind 'ConfigMap', got %q", cm.Kind)
				}
				if len(cm.Data) == 0 {
					t.Error("expected data section, got empty")
				}
				if _, ok := cm.Data["values.yaml"]; !ok {
					t.Error("expected 'values.yaml' key in data")
				}
			},
		},
		{
			name:     "load nested-values fixture",
			filename: "nested-values.yaml",
			wantErr:  false,
			validate: func(t *testing.T, cm *corev1.ConfigMap) {
				if cm.Name != "nested-values" {
					t.Errorf("expected name 'nested-values', got %q", cm.Name)
				}
				if len(cm.Data) == 0 {
					t.Error("expected data section, got empty")
				}
			},
		},
		{
			name:     "load override-values fixture",
			filename: "override-values.yaml",
			wantErr:  false,
			validate: func(t *testing.T, cm *corev1.ConfigMap) {
				if cm.Name != "override-values" {
					t.Errorf("expected name 'override-values', got %q", cm.Name)
				}
				if len(cm.Data) == 0 {
					t.Error("expected data section, got empty")
				}
			},
		},
		{
			name:     "load multi-file-values fixture",
			filename: "multi-file-values.yaml",
			wantErr:  false,
			validate: func(t *testing.T, cm *corev1.ConfigMap) {
				if cm.Name != "multi-file-values" {
					t.Errorf("expected name 'multi-file-values', got %q", cm.Name)
				}
				if len(cm.Data) == 0 {
					t.Error("expected data section, got empty")
				}
			},
		},
		{
			name:     "load special-chars-values fixture",
			filename: "special-chars-values.yaml",
			wantErr:  false,
			validate: func(t *testing.T, cm *corev1.ConfigMap) {
				if cm.Name != "special-chars-values" {
					t.Errorf("expected name 'special-chars-values', got %q", cm.Name)
				}
				if len(cm.Data) == 0 {
					t.Error("expected data section, got empty")
				}
			},
		},
		{
			name:     "load nonexistent fixture",
			filename: "nonexistent.yaml",
			wantErr:  true,
			validate: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := LoadConfigMapFixture(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigMapFixture() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, cm)
			}
		})
	}
}

func TestLoadSecretFixture(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
		validate func(t *testing.T, secret *corev1.Secret)
	}{
		{
			name:     "load database-credentials fixture",
			filename: "database-credentials.yaml",
			wantErr:  false,
			validate: func(t *testing.T, secret *corev1.Secret) {
				if secret.Name != "database-credentials" {
					t.Errorf("expected name 'database-credentials', got %q", secret.Name)
				}
				if secret.Kind != "Secret" {
					t.Errorf("expected kind 'Secret', got %q", secret.Kind)
				}
				if len(secret.Data) == 0 {
					t.Error("expected data section, got empty")
				}
			},
		},
		{
			name:     "load nonexistent fixture",
			filename: "nonexistent.yaml",
			wantErr:  true,
			validate: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret, err := LoadSecretFixture(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadSecretFixture() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, secret)
			}
		})
	}
}

func TestWithNamespace(t *testing.T) {
	tests := []struct {
		name          string
		obj           interface{}
		namespace     string
		validateError bool
	}{
		{
			name: "set namespace on ConfigMap",
			obj: &corev1.ConfigMap{
				Data: map[string]string{"key": "value"},
			},
			namespace:     "test-ns",
			validateError: false,
		},
		{
			name: "set namespace on Secret",
			obj: &corev1.Secret{
				Data: map[string][]byte{"key": []byte("value")},
			},
			namespace:     "test-ns",
			validateError: false,
		},
		{
			name:          "handle nil ConfigMap",
			obj:           (*corev1.ConfigMap)(nil),
			namespace:     "test-ns",
			validateError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WithNamespace(tt.obj, tt.namespace)

			switch v := result.(type) {
			case *corev1.ConfigMap:
				if v != nil && v.Namespace != tt.namespace {
					t.Errorf("expected namespace %q, got %q", tt.namespace, v.Namespace)
				}
			case *corev1.Secret:
				if v != nil && v.Namespace != tt.namespace {
					t.Errorf("expected namespace %q, got %q", tt.namespace, v.Namespace)
				}
			}
		})
	}
}

func TestConfigMapWithNamespace(t *testing.T) {
	tests := []struct {
		name      string
		cm        *corev1.ConfigMap
		namespace string
	}{
		{
			name: "set namespace on ConfigMap",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Data:       map[string]string{"key": "value"},
			},
			namespace: "new-ns",
		},
		{
			name:      "handle nil ConfigMap",
			cm:        nil,
			namespace: "new-ns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConfigMapWithNamespace(tt.cm, tt.namespace)

			if result != nil && result.Namespace != tt.namespace {
				t.Errorf("expected namespace %q, got %q", tt.namespace, result.Namespace)
			}
		})
	}
}

func TestSecretWithNamespace(t *testing.T) {
	tests := []struct {
		name      string
		secret    *corev1.Secret
		namespace string
	}{
		{
			name: "set namespace on Secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Data:       map[string][]byte{"key": []byte("value")},
			},
			namespace: "new-ns",
		},
		{
			name:      "handle nil Secret",
			secret:    nil,
			namespace: "new-ns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecretWithNamespace(tt.secret, tt.namespace)

			if result != nil && result.Namespace != tt.namespace {
				t.Errorf("expected namespace %q, got %q", tt.namespace, result.Namespace)
			}
		})
	}
}
