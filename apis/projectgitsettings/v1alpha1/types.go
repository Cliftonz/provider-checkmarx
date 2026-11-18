// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// +kubebuilder:validation:XValidation:rule="has(self.projectId) || has(self.projectIdRef) || has(self.projectIdSelector)",message="projectId, projectIdRef or projectIdSelector is required"
type ProjectGitSettingsParameters struct {
	// ProjectID is the UUID of the project whose git settings are managed. It
	// cannot be changed once set.
	// +crossplane:generate:reference:type=github.com/Cliftonz/provider-checkmarx/apis/project/v1alpha1.Project
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectId is immutable"
	// +optional
	ProjectID string `json:"projectId,omitempty"`

	// +optional
	ProjectIDRef *xpv2.NamespacedReference `json:"projectIdRef,omitempty"`

	// +optional
	ProjectIDSelector *xpv2.NamespacedSelector `json:"projectIdSelector,omitempty"`

	URL string `json:"url"`

	Branch string `json:"branch"`

	// PrivateKey selects a key, in a Secret in the resource's namespace,
	// holding the private key used to access the repository. The key is
	// write-only: Checkmarx never returns it, so it is applied whenever the
	// value in the Secret changes.
	PrivateKey xpv2.LocalSecretKeySelector `json:"privateKey"`
}

type ProjectGitSettingsObservation struct {
	// PrivateKeyHash is the SHA-256 of the private key last applied from the
	// Secret, used to detect changes to its content.
	PrivateKeyHash string `json:"privateKeyHash,omitempty"`
}

type ProjectGitSettingsSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              ProjectGitSettingsParameters `json:"forProvider"`
}

type ProjectGitSettingsStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`
	AtProvider                 ProjectGitSettingsObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectGitSettings is the remote git repository of a Checkmarx One project.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,checkmarx}
type ProjectGitSettings struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectGitSettingsSpec   `json:"spec"`
	Status ProjectGitSettingsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ProjectGitSettingsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProjectGitSettings `json:"items"`
}
