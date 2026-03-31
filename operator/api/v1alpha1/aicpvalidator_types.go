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

// AICPValidatorSpec defines the desired state of an AICP consensus validator.
type AICPValidatorSpec struct {
	// ClusterRef is the name of the AICPCluster this validator belongs to.
	ClusterRef string `json:"clusterRef"`

	// Image is the validator container image (defaults to the cluster image).
	// +optional
	Image string `json:"image,omitempty"`

	// GRPCPort is the port for inter-validator gRPC communication.
	// +kubebuilder:default=9091
	GRPCPort int32 `json:"grpcPort,omitempty"`

	// Storage defines the PVC configuration for validator data.
	// +optional
	Storage *ValidatorStorage `json:"storage,omitempty"`

	// KeySecretRef references a Secret containing the validator's Ed25519 private key.
	KeySecretRef SecretReference `json:"keySecretRef"`
}

// ValidatorStorage defines persistent storage for a validator.
type ValidatorStorage struct {
	// Size is the PVC size (e.g., "10Gi").
	// +kubebuilder:default="10Gi"
	Size string `json:"size,omitempty"`

	// StorageClassName is the StorageClass to use.
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`
}

// AICPValidatorStatus defines the observed state of AICPValidator.
type AICPValidatorStatus struct {
	// Ready indicates whether the validator is operational.
	Ready bool `json:"ready"`

	// DID is the validator's DID.
	// +optional
	DID string `json:"did,omitempty"`

	// LastBlockSigned is the last block number this validator signed.
	// +optional
	LastBlockSigned int64 `json:"lastBlockSigned,omitempty"`

	// SyncedToBlock is the latest block this validator has synced to.
	// +optional
	SyncedToBlock int64 `json:"syncedToBlock,omitempty"`

	// Phase indicates the validator lifecycle phase.
	// +kubebuilder:validation:Enum=Pending;Syncing;Active;Inactive;Failed
	Phase string `json:"phase"`

	// Conditions represent the latest observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterRef"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Last Block",type="integer",JSONPath=".status.lastBlockSigned"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AICPValidator is the Schema for the aicpvalidators API.
type AICPValidator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AICPValidatorSpec   `json:"spec,omitempty"`
	Status AICPValidatorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AICPValidatorList contains a list of AICPValidator.
type AICPValidatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AICPValidator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AICPValidator{}, &AICPValidatorList{})
}
