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
	Group   = "accessassignment.checkmarx.m.clifton.quest"
	Version = "v1alpha1"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}
	SchemeBuilder      = runtime.NewSchemeBuilder()
)

var (
	AccessAssignmentKind             = reflect.TypeOf(AccessAssignment{}).Name()
	AccessAssignmentGroupVersionKind = SchemeGroupVersion.WithKind(AccessAssignmentKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &AccessAssignment{}, &AccessAssignmentList{})
		metav1.AddToGroupVersion(s, SchemeGroupVersion)
		return nil
	})
}
