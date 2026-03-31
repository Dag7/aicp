/*
Copyright 2026.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AICPClusterSpec defines the desired state of an AICP cluster.
type AICPClusterSpec struct {
	// Replicas is the number of AICP server replicas.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	Replicas int32 `json:"replicas"`

	// ValidatorCount is the number of consensus validators.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	ValidatorCount int32 `json:"validatorCount"`

	// Image is the AICP server container image.
	// +kubebuilder:default="ghcr.io/aicp-protocol/aicp-server:latest"
	Image string `json:"image"`

	// PostgreSQL connection configuration.
	PostgreSQL PostgreSQLConfig `json:"postgresql"`

	// Redis connection configuration.
	Redis RedisConfig `json:"redis"`

	// TLS enables TLS for the AICP server.
	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`

	// ServiceDID is the service DID for this cluster.
	// +optional
	ServiceDID string `json:"serviceDID,omitempty"`

	// AllowedOrigins for CORS.
	// +optional
	AllowedOrigins []string `json:"allowedOrigins,omitempty"`

	// Resources defines CPU/memory resources for the server pods.
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`
}

// PostgreSQLConfig defines PostgreSQL connection settings.
type PostgreSQLConfig struct {
	// Host is the PostgreSQL hostname.
	Host string `json:"host"`

	// Port is the PostgreSQL port.
	// +kubebuilder:default=5432
	Port int32 `json:"port,omitempty"`

	// Database name.
	// +kubebuilder:default="aicp"
	Database string `json:"database,omitempty"`

	// SecretRef references a Secret containing 'username' and 'password' keys.
	SecretRef SecretReference `json:"secretRef"`
}

// RedisConfig defines Redis connection settings.
type RedisConfig struct {
	// Address is the Redis address (host:port).
	Address string `json:"address"`

	// SecretRef optionally references a Secret containing a 'password' key.
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// TLSConfig defines TLS settings.
type TLSConfig struct {
	// SecretRef references a Secret containing 'tls.crt' and 'tls.key'.
	SecretRef SecretReference `json:"secretRef"`
}

// SecretReference references a Kubernetes Secret.
type SecretReference struct {
	// Name of the Secret.
	Name string `json:"name"`
}

// ResourceRequirements mirrors a subset of corev1.ResourceRequirements.
type ResourceRequirements struct {
	// CPU request (e.g., "500m").
	// +optional
	CPURequest string `json:"cpuRequest,omitempty"`

	// Memory request (e.g., "256Mi").
	// +optional
	MemoryRequest string `json:"memoryRequest,omitempty"`

	// CPU limit (e.g., "2").
	// +optional
	CPULimit string `json:"cpuLimit,omitempty"`

	// Memory limit (e.g., "1Gi").
	// +optional
	MemoryLimit string `json:"memoryLimit,omitempty"`
}

// AICPClusterStatus defines the observed state of AICPCluster.
type AICPClusterStatus struct {
	// ReadyReplicas is the number of ready server pods.
	ReadyReplicas int32 `json:"readyReplicas"`

	// ValidatorsReady is the number of ready validator nodes.
	ValidatorsReady int32 `json:"validatorsReady"`

	// Phase indicates the overall cluster phase.
	// +kubebuilder:validation:Enum=Pending;Running;Degraded;Failed
	Phase string `json:"phase"`

	// Conditions represent the latest available observations of the cluster state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LatestBlock is the latest finalized ledger block number.
	// +optional
	LatestBlock int64 `json:"latestBlock,omitempty"`

	// ServiceDID is the resolved service DID.
	// +optional
	ServiceDID string `json:"serviceDID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Validators",type="integer",JSONPath=".spec.validatorCount"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AICPCluster is the Schema for the aicpclusters API.
type AICPCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AICPClusterSpec   `json:"spec,omitempty"`
	Status AICPClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AICPClusterList contains a list of AICPCluster.
type AICPClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AICPCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AICPCluster{}, &AICPClusterList{})
}
