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

	"github.com/Cliftonz/provider-checkmarx/apis/projectconfiguration/v1alpha1"
)

func apiServer(t *testing.T) client.Client {
	t.Helper()
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "package", "crds", "projectconfiguration.checkmarx.m.clifton.quest_projectconfigurations.yaml")},
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

var presetEntry = v1alpha1.ConfigurationEntry{Key: "scan.config.sast.presetName", Value: "ASA Premium"}

func projectConfiguration(name string, p v1alpha1.ProjectConfigurationParameters) *v1alpha1.ProjectConfiguration {
	return &v1alpha1.ProjectConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       v1alpha1.ProjectConfigurationSpec{ForProvider: p},
	}
}

func TestProjectConfigurationValidation(t *testing.T) {
	kube := apiServer(t)

	t.Run("projectId is immutable", func(t *testing.T) {
		pc := projectConfiguration("immutable-project", v1alpha1.ProjectConfigurationParameters{ProjectID: backendID, Entries: []v1alpha1.ConfigurationEntry{presetEntry}})
		if err := kube.Create(t.Context(), pc); err != nil {
			t.Fatal(err)
		}
		pc.Spec.ForProvider.ProjectID = "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
		err := kube.Update(t.Context(), pc)
		if err == nil || !strings.Contains(err.Error(), "projectId is immutable") {
			t.Errorf("update error = %v, want projectId is immutable", err)
		}
	})

	t.Run("a reference may fill projectId later", func(t *testing.T) {
		pc := projectConfiguration("by-reference", v1alpha1.ProjectConfigurationParameters{
			ProjectIDRef: &xpv2.NamespacedReference{Name: "backend"},
			Entries:      []v1alpha1.ConfigurationEntry{presetEntry},
		})
		if err := kube.Create(t.Context(), pc); err != nil {
			t.Fatal(err)
		}
		pc.Spec.ForProvider.ProjectID = backendID
		if err := kube.Update(t.Context(), pc); err != nil {
			t.Errorf("update error = %v, want none", err)
		}
	})

	t.Run("a selector satisfies the projectId rule", func(t *testing.T) {
		pc := projectConfiguration("by-selector", v1alpha1.ProjectConfigurationParameters{
			ProjectIDSelector: &xpv2.NamespacedSelector{MatchLabels: map[string]string{"app": "backend"}},
			Entries:           []v1alpha1.ConfigurationEntry{presetEntry},
		})
		if err := kube.Create(t.Context(), pc); err != nil {
			t.Errorf("create error = %v, want none", err)
		}
	})

	t.Run("projectId or a reference to it is required", func(t *testing.T) {
		err := kube.Create(t.Context(), projectConfiguration("no-project", v1alpha1.ProjectConfigurationParameters{Entries: []v1alpha1.ConfigurationEntry{presetEntry}}))
		if err == nil || !strings.Contains(err.Error(), ": projectId, projectIdRef or projectIdSelector is required") {
			t.Errorf("create error = %v, want projectId, projectIdRef or projectIdSelector is required", err)
		}
	})

	t.Run("at least one entry is required", func(t *testing.T) {
		err := kube.Create(t.Context(), projectConfiguration("no-entries", v1alpha1.ProjectConfigurationParameters{ProjectID: backendID, Entries: []v1alpha1.ConfigurationEntry{}}))
		if err == nil || !strings.Contains(err.Error(), "should have at least 1 items") {
			t.Errorf("create error = %v, want at least 1 items", err)
		}
	})

	t.Run("entry keys are unique", func(t *testing.T) {
		err := kube.Create(t.Context(), projectConfiguration("duplicate-keys", v1alpha1.ProjectConfigurationParameters{
			ProjectID: backendID,
			Entries:   []v1alpha1.ConfigurationEntry{presetEntry, {Key: presetEntry.Key, Value: "Default"}},
		}))
		if err == nil || !strings.Contains(err.Error(), "Duplicate value") {
			t.Errorf("create error = %v, want a duplicate key error", err)
		}
	})
}
