// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	Group   = "projectconfiguration.checkmarx.m.clifton.quest"
	Version = "v1alpha1"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}
	SchemeBuilder      = runtime.NewSchemeBuilder()
)

var (
	ProjectConfigurationKind             = reflect.TypeOf(ProjectConfiguration{}).Name()
	ProjectConfigurationGroupVersionKind = SchemeGroupVersion.WithKind(ProjectConfigurationKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ProjectConfiguration{}, &ProjectConfigurationList{})
		metav1.AddToGroupVersion(s, SchemeGroupVersion)
		return nil
	})
}
