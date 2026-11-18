// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	projectv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/project/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/apis/projectgitsettings/v1alpha1"
)

func TestResolveReferencesSetsProjectIDFromProjectExternalName(t *testing.T) {
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{v1alpha1.SchemeBuilder.AddToScheme, projectv1alpha1.SchemeBuilder.AddToScheme} {
		if err := add(s); err != nil {
			t.Fatal(err)
		}
	}
	p := &projectv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "team-a"}}
	meta.SetExternalName(p, "3fa85f64-5717-4562-b3fc-2c963f66afa6")
	kube := kfake.NewClientBuilder().WithScheme(s).WithObjects(p).Build()
	g := &v1alpha1.ProjectGitSettings{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "team-a"},
		Spec: v1alpha1.ProjectGitSettingsSpec{ForProvider: v1alpha1.ProjectGitSettingsParameters{
			ProjectIDRef: &xpv2.NamespacedReference{Name: "app"},
		}},
	}
	if err := g.ResolveReferences(t.Context(), kube); err != nil {
		t.Fatal(err)
	}
	if g.Spec.ForProvider.ProjectID != "3fa85f64-5717-4562-b3fc-2c963f66afa6" {
		t.Errorf("projectId = %q, want the Project's external name", g.Spec.ForProvider.ProjectID)
	}
}
