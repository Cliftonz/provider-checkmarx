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

type TenantConfigurationParameters struct {
	// Entries are the tenant configuration keys this resource manages. Keys not listed are left untouched.
	// +listType=map
	// +listMapKey=key
	// +kubebuilder:validation:MinItems=1
	Entries []ConfigurationEntry `json:"entries"`
}

type TenantConfigurationSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              TenantConfigurationParameters `json:"forProvider"`
}

type TenantConfigurationStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A TenantConfiguration manages listed configuration keys of the Checkmarx One tenant. Its external name is always "tenant".
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,checkmarx}
type TenantConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantConfigurationSpec   `json:"spec"`
	Status TenantConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type TenantConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TenantConfiguration `json:"items"`
}
