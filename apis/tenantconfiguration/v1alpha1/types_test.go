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

	"github.com/Cliftonz/provider-checkmarx/apis/tenantconfiguration/v1alpha1"
)

func TestEntriesAreRejected(t *testing.T) {
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "package", "crds", "tenantconfiguration.checkmarx.m.clifton.quest_tenantconfigurations.yaml")},
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
		entries []v1alpha1.ConfigurationEntry
		want    string
	}{
		"duplicate keys": {
			entries: []v1alpha1.ConfigurationEntry{{Key: "scan.config.sast.presetName", Value: "ASA Premium"}, {Key: "scan.config.sast.presetName", Value: "Checkmarx Default"}},
			want:    "Duplicate value",
		},
		"empty list": {
			entries: []v1alpha1.ConfigurationEntry{},
			want:    "should have at least 1 items",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cr := &v1alpha1.TenantConfiguration{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "tenant-", Namespace: "default"},
				Spec:       v1alpha1.TenantConfigurationSpec{ForProvider: v1alpha1.TenantConfigurationParameters{Entries: tc.entries}},
			}
			err := kube.Create(t.Context(), cr)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("create error = %v, want %q", err, tc.want)
			}
		})
	}
}
