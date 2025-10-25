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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Bundle phase constants
const (
	PhaseSyncing = "Syncing"
	PhaseSynced  = "Synced"
	PhaseFailed  = "Failed"
)

// WerfBundleSpec defines the desired state of WerfBundle.
// Example:
//
//	apiVersion: werf.io/v1alpha1
//	kind: WerfBundle
//	metadata:
//	  name: my-app
//	  namespace: default
//	spec:
//	  registry:
//	    url: ghcr.io/org/bundle
//	    secretRef:
//	      name: registry-creds
//	    pollInterval: 15m
//	  converge:
//	    serviceAccountName: werf-converge
type WerfBundleSpec struct {
	// Registry contains configuration for accessing the OCI registry where the bundle is stored.
	// +kubebuilder:validation:Required
	Registry RegistryConfig `json:"registry"`

	// Converge contains configuration for deploying the bundle with werf converge.
	// +kubebuilder:validation:Required
	Converge ConvergeConfig `json:"converge"`
}

// RegistryConfig contains configuration for accessing an OCI registry.
type RegistryConfig struct {
	// URL is the OCI registry URL (e.g., ghcr.io/org/bundle).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// SecretRef is an optional reference to a Secret containing registry credentials.
	// +kubebuilder:validation:Optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// PollInterval is the interval at which to poll the registry for new tags (e.g., 15m, 1h).
	// Currently not enforced; reconciliation happens on all events plus periodic resync.
	// Proper interval enforcement will be added in a future release.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^([0-9]+(ns|us|µs|ms|s|m|h))+$`
	// +kubebuilder:default:="15m"
	PollInterval string `json:"pollInterval,omitempty"`
}

// ConvergeConfig contains configuration for deploying the bundle with werf converge.
type ConvergeConfig struct {
	// ServiceAccountName is the name of the ServiceAccount to use for running werf converge Jobs.
	// This ServiceAccount must exist in the bundle's namespace with permissions to create/update resources.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ServiceAccountName string `json:"serviceAccountName"`
}

// WerfBundleStatus defines the observed state of WerfBundle.
type WerfBundleStatus struct {
	// Phase is the current phase of the bundle (Syncing, Synced, Failed).
	// +kubebuilder:validation:Enum=Syncing;Synced;Failed
	Phase string `json:"phase,omitempty"`

	// LastAppliedTag is the last successfully deployed tag.
	// +kubebuilder:validation:Optional
	LastAppliedTag string `json:"lastAppliedTag,omitempty"`

	// LastSyncTime is the timestamp of the last successful sync (nil if not yet synced).
	// +kubebuilder:validation:Optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// LastErrorMessage is the last error encountered, or empty string if no error.
	// +kubebuilder:validation:Optional
	LastErrorMessage string `json:"lastErrorMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="LastAppliedTag",type=string,JSONPath=`.status.lastAppliedTag`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=wb;wbs

// WerfBundle is the Schema for the werfbundles API.
type WerfBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WerfBundleSpec   `json:"spec,omitempty"`
	Status WerfBundleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WerfBundleList contains a list of WerfBundle.
type WerfBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WerfBundle `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WerfBundle{}, &WerfBundleList{})
}
