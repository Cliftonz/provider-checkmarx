// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"slices"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	groupv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/group/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/apis/project/v1alpha1"
)

const (
	developersID = "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d"
	auditorsID   = "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
)

func TestResolveGroupsFromExternalNames(t *testing.T) {
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		v1alpha1.SchemeBuilder.AddToScheme,
		groupv1alpha1.SchemeBuilder.AddToScheme,
	} {
		if err := add(s); err != nil {
			t.Fatal(err)
		}
	}
	developers := &groupv1alpha1.Group{ObjectMeta: metav1.ObjectMeta{Name: "developers", Namespace: "team-a"}}
	meta.SetExternalName(developers, developersID)
	auditors := &groupv1alpha1.Group{ObjectMeta: metav1.ObjectMeta{Name: "auditors", Namespace: "team-a"}}
	meta.SetExternalName(auditors, auditorsID)
	kube := kfake.NewClientBuilder().WithScheme(s).WithObjects(developers, auditors).Build()

	p := &v1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "team-a"},
		Spec: v1alpha1.ProjectSpec{ForProvider: v1alpha1.ProjectParameters{
			Name:      "app",
			GroupsRef: []xpv2.NamespacedReference{{Name: "developers"}, {Name: "auditors"}},
		}},
	}
	if err := p.ResolveReferences(t.Context(), kube); err != nil {
		t.Fatal(err)
	}
	if want := []string{developersID, auditorsID}; !slices.Equal(p.Spec.ForProvider.Groups, want) {
		t.Errorf("groups = %v, want %v", p.Spec.ForProvider.Groups, want)
	}
}
