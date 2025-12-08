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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Bundle phase constants
const (
	PhaseSyncing = "Syncing"
	PhaseSynced  = "Synced"
	PhaseFailed  = "Failed"
)

// Job status constants
const (
	JobStatusRunning   = "Running"
	JobStatusSucceeded = "Succeeded"
	JobStatusFailed    = "Failed"
)

// WerfBundleSpec defines the desired state of WerfBundle.
//
// Example (same-namespace deployment):
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
//
// Example (cross-namespace deployment):
//
//	apiVersion: werf.io/v1alpha1
//	kind: WerfBundle
//	metadata:
//	  name: my-app
//	  namespace: operator-system
//	spec:
//	  registry:
//	    url: ghcr.io/org/bundle
//	    secretRef:
//	      name: registry-creds
//	  converge:
//	    targetNamespace: my-app-prod
//	    serviceAccountName: werf-deploy
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
	// Required for cross-namespace deployments (when TargetNamespace differs from bundle namespace).
	// Optional for same-namespace deployments (backward compatibility with default ServiceAccount).
	// When specified, the ServiceAccount must exist in the target namespace.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// TargetNamespace is the namespace where werf converge will deploy resources.
	// If not specified, defaults to the bundle's namespace.
	// This is also used as the fallback namespace when looking up values from ConfigMaps and Secrets.
	// +kubebuilder:validation:Optional
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// ResourceLimits specifies CPU and memory limits for werf converge jobs.
	// If not specified, defaults are used: 1 CPU and 1Gi memory.
	// +kubebuilder:validation:Optional
	ResourceLimits *ResourceLimitsConfig `json:"resourceLimits,omitempty"`

	// LogRetentionDays specifies how many days completed jobs should be retained.
	// After this period, jobs are automatically cleaned up by Kubernetes.
	// If not specified, defaults to 7 days.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:=7
	LogRetentionDays *int32 `json:"logRetentionDays,omitempty"`

	// ValuesFrom is a list of sources to populate configuration values for werf converge.
	// Each source is treated as a YAML document and merged in array order.
	// Later sources take precedence over earlier ones in case of key conflicts.
	// Each entry must specify exactly one of ConfigMapRef or SecretRef.
	// +kubebuilder:validation:Optional
	ValuesFrom []ValuesSource `json:"valuesFrom,omitempty"`
}

// ResourceLimitsConfig specifies CPU and memory limits for jobs.
type ResourceLimitsConfig struct {
	// CPU is the CPU limit as a string (e.g., "500m", "2", "1.5").
	// +kubebuilder:validation:Optional
	CPU string `json:"cpu,omitempty"`

	// Memory is the memory limit as a string (e.g., "512Mi", "1Gi", "2G").
	// +kubebuilder:validation:Optional
	Memory string `json:"memory,omitempty"`
}

// ValuesSource represents a source of configuration values for werf converge.
// The entire ConfigMap or Secret is treated as YAML data and merged with other sources.
// Values are passed to werf converge as --set flags.
// Exactly one of ConfigMapRef or SecretRef must be set.
// +kubebuilder:validation:XValidation:rule="(has(self.configMapRef) && !has(self.secretRef)) || (!has(self.configMapRef) && has(self.secretRef))",message="exactly one of configMapRef or secretRef must be set"
type ValuesSource struct {
	// ConfigMapRef is a reference to a ConfigMap containing values as YAML data.
	// The ConfigMap is looked up first in the WerfBundle's namespace, then in the target namespace.
	// +kubebuilder:validation:Optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// SecretRef is a reference to a Secret containing values as YAML data.
	// The Secret is looked up first in the WerfBundle's namespace, then in the target namespace.
	// +kubebuilder:validation:Optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// Optional specifies whether this values source is required.
	// If false (default), the deployment fails if the ConfigMap or Secret is not found.
	// If true, the deployment proceeds even if the resource is missing.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=false
	Optional bool `json:"optional,omitempty"`
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

	// LastETag is the ETag from the last successful registry response.
	// Used for ETag-based caching to avoid re-downloading unchanged tag lists.
	// +kubebuilder:validation:Optional
	LastETag string `json:"lastETag,omitempty"`

	// ConsecutiveFailures is the number of consecutive registry polling failures.
	// Used to calculate exponential backoff. Reset to 0 on success.
	// Marked Failed if ConsecutiveFailures > 5 (after 6th consecutive error).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=6
	ConsecutiveFailures int32 `json:"consecutiveFailures,omitempty"`

	// LastErrorTime is the timestamp of the last error encountered.
	// Used to calculate backoff intervals for retries.
	// +kubebuilder:validation:Optional
	LastErrorTime *metav1.Time `json:"lastErrorTime,omitempty"`

	// ActiveJobName is the name of the currently active job running werf converge.
	// Set when a job is created, cleared when the job completes or fails.
	// Used for deduplication to prevent multiple jobs for the same bundle version.
	// +kubebuilder:validation:Optional
	ActiveJobName string `json:"activeJobName,omitempty"`

	// LastJobStatus is the status of the most recent job (Succeeded, Failed, Running).
	// Provides quick visibility into the last deployment attempt without listing jobs separately.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=Succeeded;Failed;Running
	LastJobStatus string `json:"lastJobStatus,omitempty"`

	// LastJobLogs are the captured logs from the most recent job (tail of output).
	// Limited to ~5KB to fit in Status; larger logs are stored in a ConfigMap instead.
	// Provides debugging visibility without requiring external log aggregation.
	// +kubebuilder:validation:Optional
	LastJobLogs string `json:"lastJobLogs,omitempty"`

	// ResolvedTargetNamespace is the namespace where the bundle is deployed.
	// Defaults to bundle namespace if TargetNamespace is not set in spec.
	// Provides visibility for debugging cross-namespace deployments.
	// +kubebuilder:validation:Optional
	ResolvedTargetNamespace string `json:"resolvedTargetNamespace,omitempty"`
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

// ValidateCrossNamespaceDeployment checks if ServiceAccountName is required for cross-namespace deployment.
// Returns an error if TargetNamespace differs from bundle namespace but ServiceAccountName is not set.
// For same-namespace deployments (TargetNamespace empty or equals bundle namespace), ServiceAccountName is optional.
func (wb *WerfBundle) ValidateCrossNamespaceDeployment() error {
	targetNs := wb.Spec.Converge.TargetNamespace
	bundleNs := wb.Namespace
	saName := wb.Spec.Converge.ServiceAccountName

	// If TargetNamespace is empty or equals bundle namespace, it's same-namespace deployment
	if targetNs == "" || targetNs == bundleNs {
		return nil
	}

	// Cross-namespace deployment requires ServiceAccountName
	if saName == "" {
		return fmt.Errorf(
			"serviceAccountName is required for cross-namespace deployment from %s to %s",
			bundleNs, targetNs,
		)
	}

	return nil
}

func init() {
	SchemeBuilder.Register(&WerfBundle{}, &WerfBundleList{})
}
