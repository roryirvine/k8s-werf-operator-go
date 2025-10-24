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

// WerfBundleSpec defines the desired state of WerfBundle.
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

	// SecretRef is a reference to a Secret containing registry credentials.
	// The Secret should have keys: username, password, or be mountable as Docker config.
	// +kubebuilder:validation:Optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// PollInterval is the interval at which to poll the registry for new tags.
	// Format: 15m, 1h, etc. (Duration format). Default: 15m
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^([0-9]+(ns|us|µs|ms|s|m|h))+$`
	// +kubebuilder:default:="15m"
	PollInterval string `json:"pollInterval,omitempty"`

	// VersionConstraint is a semver constraint for selecting tags (e.g., >=1.0.0, ~1.2.x).
	// If unset, uses the latest tag.
	// +kubebuilder:validation:Optional
	VersionConstraint *string `json:"versionConstraint,omitempty"`
}

// ConvergeConfig contains configuration for running werf converge.
type ConvergeConfig struct {
	// TargetNamespace is the namespace where the bundle will be deployed.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	TargetNamespace string `json:"targetNamespace"`

	// ServiceAccountName is the name of the ServiceAccount to use for running werf converge.
	// This account must exist in the target namespace and have permissions to create/update resources.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ServiceAccountName string `json:"serviceAccountName"`

	// ResourceLimits specifies CPU and memory limits for the converge job.
	// +kubebuilder:validation:Optional
	ResourceLimits *ResourceLimits `json:"resourceLimits,omitempty"`

	// LogRetentionDays specifies how many days to retain job logs. Default: 7
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=365
	LogRetentionDays *int32 `json:"logRetentionDays,omitempty"`

	// DriftDetection configures periodic drift detection and correction.
	// +kubebuilder:validation:Optional
	DriftDetection *DriftDetectionConfig `json:"driftDetection,omitempty"`

	// ValuesFrom is a list of sources for values to pass to werf converge.
	// Sources are processed in order, with later values overriding earlier ones.
	// +kubebuilder:validation:Optional
	ValuesFrom []ValuesSource `json:"valuesFrom,omitempty"`
}

// ResourceLimits specifies CPU and memory limits.
type ResourceLimits struct {
	// CPU is the CPU limit (e.g., "1", "500m").
	// +kubebuilder:validation:Optional
	CPU *string `json:"cpu,omitempty"`

	// Memory is the memory limit (e.g., "1Gi", "512Mi").
	// +kubebuilder:validation:Optional
	Memory *string `json:"memory,omitempty"`
}

// DriftDetectionConfig configures periodic drift detection.
type DriftDetectionConfig struct {
	// Enabled specifies whether drift detection is enabled.
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// Interval is how often to check for drift. Format: 15m, 1h, etc.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^([0-9]+(ns|us|µs|ms|s|m|h))+$`
	// +kubebuilder:default:="15m"
	Interval string `json:"interval,omitempty"`

	// MaxRetries is the maximum number of retry attempts for failed drift checks.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	MaxRetries *int32 `json:"maxRetries,omitempty"`
}

// ValuesSource specifies a source for values to pass to werf converge.
type ValuesSource struct {
	// ConfigMapRef is a reference to a ConfigMap containing values.
	// +kubebuilder:validation:Optional
	ConfigMapRef *ConfigMapRef `json:"configMapRef,omitempty"`

	// SecretRef is a reference to a Secret containing values.
	// +kubebuilder:validation:Optional
	SecretRef *SecretRef `json:"secretRef,omitempty"`
}

// ConfigMapRef is a reference to a ConfigMap with optional fields.
type ConfigMapRef struct {
	// Name is the name of the ConfigMap.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Optional indicates whether the ConfigMap must exist.
	// +kubebuilder:validation:Optional
	Optional *bool `json:"optional,omitempty"`
}

// SecretRef is a reference to a Secret with optional fields.
type SecretRef struct {
	// Name is the name of the Secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Optional indicates whether the Secret must exist.
	// +kubebuilder:validation:Optional
	Optional *bool `json:"optional,omitempty"`
}

// WerfBundleStatus defines the observed state of WerfBundle.
type WerfBundleStatus struct {
	// Phase is the current phase of the bundle (Syncing, Synced, Failed).
	// +kubebuilder:validation:Enum=Syncing;Synced;Failed
	Phase string `json:"phase,omitempty"`

	// LastAppliedTag is the last successfully deployed tag.
	// +kubebuilder:validation:Optional
	LastAppliedTag string `json:"lastAppliedTag,omitempty"`

	// LastSyncTime is the timestamp of the last successful sync.
	// +kubebuilder:validation:Optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// LastErrorMessage contains the last error message, if any.
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
