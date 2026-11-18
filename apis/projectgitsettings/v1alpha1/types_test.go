// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build envtest

package v1alpha1_test

import (
	"path/filepath"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/Cliftonz/provider-checkmarx/apis/projectgitsettings/v1alpha1"
)

func newKube(t *testing.T) client.Client {
	t.Helper()
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "package", "crds", "projectgitsettings.checkmarx.m.clifton.quest_projectgitsettings.yaml")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := env.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = env.Stop() })
	s := runtime.NewScheme()
	if err := v1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	kube, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		t.Fatal(err)
	}
	return kube
}

func gitSettings(name string, fp v1alpha1.ProjectGitSettingsParameters) *v1alpha1.ProjectGitSettings {
	fp.URL = "https://github.com/acme/app.git"
	fp.Branch = "main"
	fp.PrivateKey = xpv2.LocalSecretKeySelector{LocalSecretReference: xpv2.LocalSecretReference{Name: "app-deploy-key"}, Key: "privateKey"}
	return &v1alpha1.ProjectGitSettings{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       v1alpha1.ProjectGitSettingsSpec{ForProvider: fp},
	}
}

func TestCRD(t *testing.T) {
	kube := newKube(t)

	t.Run("ProjectIsRequired", func(t *testing.T) {
		err := kube.Create(t.Context(), gitSettings("no-project", v1alpha1.ProjectGitSettingsParameters{}))
		if err == nil || !strings.Contains(err.Error(), "projectId, projectIdRef or projectIdSelector is required") {
			t.Errorf("create error = %v, want project to be required", err)
		}
	})

	t.Run("ProjectIDRefOrSelectorSatisfiesProject", func(t *testing.T) {
		for name, fp := range map[string]v1alpha1.ProjectGitSettingsParameters{
			"by-id":       {ProjectID: "3fa85f64-5717-4562-b3fc-2c963f66afa6"},
			"by-ref":      {ProjectIDRef: &xpv2.NamespacedReference{Name: "app"}},
			"by-selector": {ProjectIDSelector: &xpv2.NamespacedSelector{MatchLabels: map[string]string{"app": "app"}}},
		} {
			if err := kube.Create(t.Context(), gitSettings(name, fp)); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	})

	t.Run("ProjectIDIsImmutable", func(t *testing.T) {
		g := gitSettings("immutable", v1alpha1.ProjectGitSettingsParameters{ProjectID: "3fa85f64-5717-4562-b3fc-2c963f66afa6"})
		if err := kube.Create(t.Context(), g); err != nil {
			t.Fatal(err)
		}
		g.Spec.ForProvider.ProjectID = "9b2c1d4e-0000-4000-8000-000000000000"
		err := kube.Update(t.Context(), g)
		if err == nil || !strings.Contains(err.Error(), "projectId is immutable") {
			t.Errorf("update error = %v, want projectId is immutable", err)
		}
	})

	t.Run("ProjectIDCanBeSetByResolution", func(t *testing.T) {
		g := gitSettings("resolved", v1alpha1.ProjectGitSettingsParameters{ProjectIDRef: &xpv2.NamespacedReference{Name: "app"}})
		if err := kube.Create(t.Context(), g); err != nil {
			t.Fatal(err)
		}
		g.Spec.ForProvider.ProjectID = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
		if err := kube.Update(t.Context(), g); err != nil {
			t.Errorf("update error = %v, want the resolved projectId accepted", err)
		}
	})
}
