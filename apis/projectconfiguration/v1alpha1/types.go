// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

type ConfigurationEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// +kubebuilder:validation:XValidation:rule="has(self.projectId) || has(self.projectIdRef) || has(self.projectIdSelector)",message="projectId, projectIdRef or projectIdSelector is required"
type ProjectConfigurationParameters struct {
	// ProjectID is the UUID of the Checkmarx One project whose configuration is managed. It cannot be changed.
	// +crossplane:generate:reference:type=github.com/Cliftonz/provider-checkmarx/apis/project/v1alpha1.Project
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectId is immutable"
	// +optional
	ProjectID string `json:"projectId,omitempty"`

	// +optional
	ProjectIDRef *xpv2.NamespacedReference `json:"projectIdRef,omitempty"`

	// +optional
	ProjectIDSelector *xpv2.NamespacedSelector `json:"projectIdSelector,omitempty"`

	// Entries are the configuration keys to manage; keys not listed are left untouched.
	// +listType=map
	// +listMapKey=key
	// +kubebuilder:validation:MinItems=1
	Entries []ConfigurationEntry `json:"entries"`
}

type ProjectConfigurationSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              ProjectConfigurationParameters `json:"forProvider"`
}

type ProjectConfigurationStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A ProjectConfiguration manages listed configuration keys of a Checkmarx One project.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,checkmarx}
type ProjectConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectConfigurationSpec   `json:"spec"`
	Status ProjectConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ProjectConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectConfiguration `json:"items"`
}
