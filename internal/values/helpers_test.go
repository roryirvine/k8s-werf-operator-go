package values

import (
	"testing"

	werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"
)

func TestGetTargetNamespace(t *testing.T) {
	tests := []struct {
		name            string
		convergeConfig  *werfv1alpha1.ConvergeConfig
		bundleNamespace string
		want            string
	}{
		{
			name: "TargetNamespace specified returns it",
			convergeConfig: &werfv1alpha1.ConvergeConfig{
				TargetNamespace: "custom-namespace",
			},
			bundleNamespace: "bundle-ns",
			want:            "custom-namespace",
		},
		{
			name: "Empty TargetNamespace returns bundle namespace",
			convergeConfig: &werfv1alpha1.ConvergeConfig{
				TargetNamespace: "",
			},
			bundleNamespace: "bundle-ns",
			want:            "bundle-ns",
		},
		{
			name:            "Nil ConvergeConfig returns bundle namespace",
			convergeConfig:  &werfv1alpha1.ConvergeConfig{},
			bundleNamespace: "bundle-ns",
			want:            "bundle-ns",
		},
		{
			name: "Same as bundle namespace",
			convergeConfig: &werfv1alpha1.ConvergeConfig{
				TargetNamespace: "bundle-ns",
			},
			bundleNamespace: "bundle-ns",
			want:            "bundle-ns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTargetNamespace(tt.convergeConfig, tt.bundleNamespace)
			if got != tt.want {
				t.Errorf("GetTargetNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateSetFlags(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   []string
	}{
		{
			name:   "Empty map returns nil",
			values: map[string]string{},
			want:   nil,
		},
		{
			name:   "Nil map returns nil",
			values: nil,
			want:   nil,
		},
		{
			name: "Single key-value pair",
			values: map[string]string{
				"key1": "value1",
			},
			want: []string{"--set", "key1=value1"},
		},
		{
			name: "Multiple key-value pairs are sorted",
			values: map[string]string{
				"zebra": "z-value",
				"alpha": "a-value",
				"beta":  "b-value",
			},
			want: []string{
				"--set", "alpha=a-value",
				"--set", "beta=b-value",
				"--set", "zebra=z-value",
			},
		},
		{
			name: "Dot notation keys are sorted",
			values: map[string]string{
				"app.database.host": "localhost",
				"app.cache.enabled": "true",
				"app.database.port": "5432",
			},
			want: []string{
				"--set", "app.cache.enabled=true",
				"--set", "app.database.host=localhost",
				"--set", "app.database.port=5432",
			},
		},
		{
			name: "Array bracket notation keys",
			values: map[string]string{
				"servers[0].name": "server1",
				"servers[1].name": "server2",
				"servers[0].port": "8080",
			},
			want: []string{
				"--set", "servers[0].name=server1",
				"--set", "servers[0].port=8080",
				"--set", "servers[1].name=server2",
			},
		},
		{
			name: "Values with special characters",
			values: map[string]string{
				"key1": "value with spaces",
				"key2": "value=with=equals",
				"key3": "value,with,commas",
			},
			want: []string{
				"--set", "key1=value with spaces",
				"--set", `key2=value\=with\=equals`,
				"--set", `key3=value\,with\,commas`,
			},
		},
		{
			name: "Empty string value",
			values: map[string]string{
				"key1": "",
			},
			want: []string{"--set", "key1="},
		},
		{
			name: "Integration test with complex special characters",
			values: map[string]string{
				"db.url":      "postgres://user:pass@localhost/db",
				"api.key":     `secret="value"`,
				"config.path": `C:\Program Files\App`,
				"list":        "item1,item2,item3",
			},
			want: []string{
				"--set", `api.key=secret\="value"`,
				"--set", `config.path=C:\\Program Files\\App`,
				"--set", "db.url=postgres://user:pass@localhost/db",
				"--set", `list=item1\,item2\,item3`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSetFlags(tt.values)
			if !slicesEqual(got, tt.want) {
				t.Errorf("GenerateSetFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

// slicesEqual compares two string slices for equality.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
