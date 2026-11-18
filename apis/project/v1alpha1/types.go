// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

type ProjectParameters struct {
	// Name of the project. It cannot be changed.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// +optional
	RepoURL *string `json:"repoUrl,omitempty"`

	// +optional
	MainBranch *string `json:"mainBranch,omitempty"`

	// +optional
	Origin *string `json:"origin,omitempty"`

	// +optional
	ScmRepoID *string `json:"scmRepoId,omitempty"`

	// +optional
	Tags map[string]string `json:"tags,omitempty"`

	// Groups are the UUIDs of the IAM groups given access to the project.
	// +crossplane:generate:reference:type=github.com/Cliftonz/provider-checkmarx/apis/group/v1alpha1.Group
	// +crossplane:generate:reference:refFieldName=GroupsRef
	// +crossplane:generate:reference:selectorFieldName=GroupsSelector
	// +kubebuilder:validation:items:Format=uuid
	// +optional
	Groups []string `json:"groups,omitempty"`

	// +optional
	GroupsRef []xpv2.NamespacedReference `json:"groupsRef,omitempty"`

	// +optional
	GroupsSelector *xpv2.NamespacedSelector `json:"groupsSelector,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	// +optional
	Criticality *int `json:"criticality,omitempty"`

	// PrivatePackage is write-only: sent on create and update, never returned by the API, so remote changes are not detected.
	// +optional
	PrivatePackage *bool `json:"privatePackage,omitempty"`
}

type ProjectObservation struct {
	ID             string       `json:"id,omitempty"`
	CreatedAt      *metav1.Time `json:"createdAt,omitempty"`
	UpdatedAt      *metav1.Time `json:"updatedAt,omitempty"`
	ApplicationIDs []string     `json:"applicationIds,omitempty"`
}

type ProjectSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              ProjectParameters `json:"forProvider"`
}

type ProjectStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`
	AtProvider                 ProjectObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Project is a Checkmarx One project.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,checkmarx}
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec"`
	Status ProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}
