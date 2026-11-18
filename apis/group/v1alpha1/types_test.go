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

	"github.com/Cliftonz/provider-checkmarx/apis/group/v1alpha1"
)

func startAPIServer(t *testing.T) client.Client {
	t.Helper()
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "package", "crds", "group.checkmarx.m.clifton.quest_groups.yaml")},
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

func groupNamed(name string) *v1alpha1.Group {
	return &v1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "devs", Namespace: "default"},
		Spec:       v1alpha1.GroupSpec{ForProvider: v1alpha1.GroupParameters{Name: name}},
	}
}

func TestNameIsImmutable(t *testing.T) {
	kube := startAPIServer(t)
	g := groupNamed("devs")
	if err := kube.Create(t.Context(), g); err != nil {
		t.Fatal(err)
	}
	g.Spec.ForProvider.Name = "ops"
	err := kube.Update(t.Context(), g)
	if err == nil || !strings.Contains(err.Error(), "name is immutable") {
		t.Errorf("update error = %v, want name is immutable", err)
	}
}

func TestEmptyNameIsRejected(t *testing.T) {
	kube := startAPIServer(t)
	err := kube.Create(t.Context(), groupNamed(""))
	if err == nil || !strings.Contains(err.Error(), "spec.forProvider.name") {
		t.Errorf("create error = %v, want spec.forProvider.name rejected", err)
	}
}
