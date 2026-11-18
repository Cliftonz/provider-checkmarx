// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

type Role struct {
	// +optional
	// +kubebuilder:validation:Format=uuid
	ID string `json:"id,omitempty"`

	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

type AccessAssignmentParameters struct {
	// EntityID is the UUID of the group or user being granted access.
	// +kubebuilder:validation:Format=uuid
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="entityID is immutable"
	EntityID string `json:"entityID"`

	// EntityType is the kind of entity. Documented values: group, user.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="entityType is immutable"
	EntityType string `json:"entityType"`

	// EntityRoles are the roles granted. Roles are compared by name.
	// +kubebuilder:validation:MinItems=1
	EntityRoles []Role `json:"entityRoles"`

	// ResourceType is the kind of resource access is granted on. Documented values: tenant, application, project.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceType is immutable"
	ResourceType string `json:"resourceType"`

	// ResourceID is the UUID of the resource access is granted on.
	// +kubebuilder:validation:Format=uuid
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID string `json:"resourceID"`
}

type AccessAssignmentSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              AccessAssignmentParameters `json:"forProvider"`
}

type AccessAssignmentStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// An AccessAssignment grants a Checkmarx One group or user roles on a tenant, application or project.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,checkmarx}
type AccessAssignment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AccessAssignmentSpec   `json:"spec"`
	Status AccessAssignmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type AccessAssignmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessAssignment `json:"items"`
}
