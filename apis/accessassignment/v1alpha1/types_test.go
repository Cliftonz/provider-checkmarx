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

	"github.com/Cliftonz/provider-checkmarx/apis/accessassignment/v1alpha1"
)

func TestIdentityFieldsAreImmutableAndRolesAreNot(t *testing.T) {
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "package", "crds", "accessassignment.checkmarx.m.clifton.quest_accessassignments.yaml")},
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

	cases := map[string]struct {
		change  func(*v1alpha1.AccessAssignmentParameters)
		message string
	}{
		"EntityID":     {change: func(p *v1alpha1.AccessAssignmentParameters) { p.EntityID = "9b2e8f10-1c4d-4e5a-8f6b-7c8d9e0f1a2b" }, message: "entityID is immutable"},
		"EntityType":   {change: func(p *v1alpha1.AccessAssignmentParameters) { p.EntityType = "user" }, message: "entityType is immutable"},
		"ResourceID":   {change: func(p *v1alpha1.AccessAssignmentParameters) { p.ResourceID = "9b2e8f10-1c4d-4e5a-8f6b-7c8d9e0f1a2b" }, message: "resourceID is immutable"},
		"ResourceType": {change: func(p *v1alpha1.AccessAssignmentParameters) { p.ResourceType = "application" }, message: "resourceType is immutable"},
		"EntityRoles":  {change: func(p *v1alpha1.AccessAssignmentParameters) { p.EntityRoles = []v1alpha1.Role{{Name: "ast-viewer"}} }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := &v1alpha1.AccessAssignment{
				ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(name), Namespace: "default"},
				Spec: v1alpha1.AccessAssignmentSpec{ForProvider: v1alpha1.AccessAssignmentParameters{
					EntityID:     "3fa85f64-5717-4562-b3fc-2c963f66afa6",
					EntityType:   "group",
					EntityRoles:  []v1alpha1.Role{{Name: "ast-scanner"}},
					ResourceType: "project",
					ResourceID:   "c56a4180-65aa-42ec-a945-5fd21dec0538",
				}},
			}
			if err := kube.Create(t.Context(), a); err != nil {
				t.Fatal(err)
			}
			tc.change(&a.Spec.ForProvider)
			err := kube.Update(t.Context(), a)
			switch {
			case tc.message == "" && err != nil:
				t.Errorf("update error = %v, want the update to succeed", err)
			case tc.message != "" && (err == nil || !strings.Contains(err.Error(), tc.message)):
				t.Errorf("update error = %v, want %s", err, tc.message)
			}
		})
	}
}

func TestEmptyRolesAndRoleNamesAreRejected(t *testing.T) {
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "package", "crds", "accessassignment.checkmarx.m.clifton.quest_accessassignments.yaml")},
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

	cases := map[string]struct {
		roles   []v1alpha1.Role
		message string
	}{
		"EmptyList": {roles: []v1alpha1.Role{}, message: "should have at least 1 items"},
		"EmptyName": {roles: []v1alpha1.Role{{Name: ""}}, message: "should be at least 1 chars long"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := &v1alpha1.AccessAssignment{
				ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(name), Namespace: "default"},
				Spec: v1alpha1.AccessAssignmentSpec{ForProvider: v1alpha1.AccessAssignmentParameters{
					EntityID:     "3fa85f64-5717-4562-b3fc-2c963f66afa6",
					EntityType:   "group",
					EntityRoles:  tc.roles,
					ResourceType: "project",
					ResourceID:   "c56a4180-65aa-42ec-a945-5fd21dec0538",
				}},
			}
			err := kube.Create(t.Context(), a)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Errorf("create error = %v, want %s", err, tc.message)
			}
		})
	}
}
