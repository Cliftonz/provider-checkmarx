// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	projectv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/project/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/apis/projectconfiguration/v1alpha1"
)

const backendID = "0b6f3a52-6a0e-4c1e-9d5e-2f7c1a9e8b10"

func TestResolveProjectIDFromExternalName(t *testing.T) {
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		v1alpha1.SchemeBuilder.AddToScheme,
		projectv1alpha1.SchemeBuilder.AddToScheme,
	} {
		if err := add(s); err != nil {
			t.Fatal(err)
		}
	}
	p := &projectv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "team-a"}}
	meta.SetExternalName(p, backendID)
	kube := kfake.NewClientBuilder().WithScheme(s).WithObjects(p).Build()

	pc := &v1alpha1.ProjectConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-config", Namespace: "team-a"},
		Spec: v1alpha1.ProjectConfigurationSpec{ForProvider: v1alpha1.ProjectConfigurationParameters{
			ProjectIDRef: &xpv2.NamespacedReference{Name: "backend"},
			Entries:      []v1alpha1.ConfigurationEntry{{Key: "scan.config.sast.presetName", Value: "ASA Premium"}},
		}},
	}
	if err := pc.ResolveReferences(t.Context(), kube); err != nil {
		t.Fatal(err)
	}
	if pc.Spec.ForProvider.ProjectID != backendID {
		t.Errorf("projectId = %q, want %q", pc.Spec.ForProvider.ProjectID, backendID)
	}
}
