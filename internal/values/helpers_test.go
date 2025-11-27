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
