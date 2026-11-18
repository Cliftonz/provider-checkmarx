// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

type ApplicationRule struct {
	// Type of the rule, such as project.name.in.
	Type string `json:"type"`

	// Value the rule matches.
	Value string `json:"value"`
}

type ApplicationParameters struct {
	// Name of the application. It cannot be changed.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// Description is late-initialized from the remote application when omitted.
	// +optional
	Description *string `json:"description,omitempty"`

	// Criticality is late-initialized from the remote application when omitted.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	// +optional
	Criticality *int `json:"criticality,omitempty"`

	// Rules that attach projects to the application. Late-initialized from the remote application when omitted.
	// +optional
	Rules []ApplicationRule `json:"rules,omitempty"`

	// Tags is late-initialized from the remote application when omitted.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`

	// Type is late-initialized from the remote application when omitted.
	// +optional
	Type *string `json:"type,omitempty"`
}

type ObservedApplicationRule struct {
	ID    string `json:"id,omitempty"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

type ApplicationObservation struct {
	ID         string                    `json:"id,omitempty"`
	Rules      []ObservedApplicationRule `json:"rules,omitempty"`
	ProjectIDs []string                  `json:"projectIds,omitempty"`
	CreatedAt  *metav1.Time              `json:"createdAt,omitempty"`
	UpdatedAt  *metav1.Time              `json:"updatedAt,omitempty"`
}

type ApplicationSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              ApplicationParameters `json:"forProvider"`
}

type ApplicationStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`
	AtProvider                 ApplicationObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// An Application is a Checkmarx One application grouping projects by rules.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,checkmarx}
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec"`
	Status ApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}
