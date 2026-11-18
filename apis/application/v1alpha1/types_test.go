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

	"github.com/Cliftonz/provider-checkmarx/apis/application/v1alpha1"
)

func startKube(t *testing.T) client.Client {
	t.Helper()
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "package", "crds", "application.checkmarx.m.clifton.quest_applications.yaml")},
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
	a := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "payments", Namespace: "default"},
		Spec:       v1alpha1.ApplicationSpec{ForProvider: v1alpha1.ApplicationParameters{Name: "payments"}},
	}
	if err := kube.Create(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	a.Spec.ForProvider.Name = "billing"
	err := kube.Update(t.Context(), a)
	if err == nil || !strings.Contains(err.Error(), "name is immutable") {
		t.Errorf("update error = %v, want name is immutable", err)
	}
}

func TestCriticalityOutsideOneToFiveIsRejected(t *testing.T) {
	kube := startKube(t)
	for _, criticality := range []int{0, 6} {
		a := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("criticality-%d", criticality), Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{ForProvider: v1alpha1.ApplicationParameters{Name: "payments", Criticality: &criticality}},
		}
		err := kube.Create(t.Context(), a)
		if err == nil || !strings.Contains(err.Error(), "spec.forProvider.criticality") {
			t.Errorf("criticality %d: create error = %v, want a spec.forProvider.criticality rejection", criticality, err)
		}
	}
}
