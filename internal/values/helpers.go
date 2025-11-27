// Package values provides utilities for resolving configuration values from ConfigMaps and Secrets.
package values

import werfv1alpha1 "github.com/werf/k8s-werf-operator-go/api/v1alpha1"

// GetTargetNamespace returns the target namespace from ConvergeConfig, or the bundle namespace if not specified.
// This implements the fallback behavior where targetNamespace defaults to bundleNamespace.
func GetTargetNamespace(convergeConfig *werfv1alpha1.ConvergeConfig, bundleNamespace string) string {
	if convergeConfig.TargetNamespace != "" {
		return convergeConfig.TargetNamespace
	}
	return bundleNamespace
}
