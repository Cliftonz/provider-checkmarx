// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build envtest

package v1alpha1_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/Cliftonz/provider-checkmarx/apis/project/v1alpha1"
)

func startKube(t *testing.T) client.Client {
	t.Helper()
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "package", "crds", "project.checkmarx.m.clifton.quest_projects.yaml")},
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

func TestNameIsImmutable(t *testing.T) {
	kube := startKube(t)
	p := &v1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "prod", Namespace: "default"},
		Spec:       v1alpha1.ProjectSpec{ForProvider: v1alpha1.ProjectParameters{Name: "prod"}},
	}
	if err := kube.Create(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	p.Spec.ForProvider.Name = "staging"
	err := kube.Update(t.Context(), p)
	if err == nil || !strings.Contains(err.Error(), "name is immutable") {
		t.Errorf("update error = %v, want name is immutable", err)
	}
}

func TestCriticalityIsBoundedOneToFive(t *testing.T) {
	kube := startKube(t)
	for _, criticality := range []int{0, 6} {
		t.Run(fmt.Sprint(criticality), func(t *testing.T) {
			p := &v1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("criticality-%d", criticality), Namespace: "default"},
				Spec:       v1alpha1.ProjectSpec{ForProvider: v1alpha1.ProjectParameters{Name: "app", Criticality: &criticality}},
			}
			err := kube.Create(t.Context(), p)
			if err == nil || !strings.Contains(err.Error(), "spec.forProvider.criticality") {
				t.Errorf("create error = %v, want criticality %d rejected", err, criticality)
			}
		})
	}
}
