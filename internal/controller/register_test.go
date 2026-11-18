// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/providerconfig"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/Cliftonz/provider-checkmarx/apis"
	accessassignmentv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/accessassignment/v1alpha1"
	applicationv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/application/v1alpha1"
	groupv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/group/v1alpha1"
	projectv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/project/v1alpha1"
	projectconfigurationv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/projectconfiguration/v1alpha1"
	projectgitsettingsv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/projectgitsettings/v1alpha1"
	tenantconfigurationv1alpha1 "github.com/Cliftonz/provider-checkmarx/apis/tenantconfiguration/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/apis/v1beta1"
)

func newManager(t *testing.T) manager.Manager {
	t.Helper()
	s := runtime.NewScheme()
	if err := apis.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	mgr, err := manager.New(&rest.Config{Host: "http://127.0.0.1:1"}, manager.Options{
		Scheme:                 s,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		t.Fatal(err)
	}
	return mgr
}

func options(gate controller.Gate) controller.Options {
	o := controller.DefaultOptions()
	o.Logger = logging.NewNopLogger()
	o.MetricOptions = &controller.MetricOptions{}
	o.Gate = gate
	return o
}

func TestSetupRegistersConfigAndKindControllers(t *testing.T) {
	mgr := newManager(t)

	if err := Setup(mgr, options(nil)); err != nil {
		t.Fatalf("Setup() unexpected error: %v", err)
	}
	for _, name := range []string{
		providerconfig.ControllerName(v1beta1.ProviderConfigGroupKind),
		providerconfig.ControllerName(v1beta1.ClusterProviderConfigGroupKind),
		managed.ControllerName(projectv1alpha1.ProjectGroupVersionKind.GroupKind().String()),
	} {
		_, err := crcontroller.New(name, mgr, crcontroller.Options{Reconciler: reconcile.Func(nil)})
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("registering %q after Setup() error = %v, want a duplicate controller", name, err)
		}
	}
}

type recordingGate struct {
	controller.Gate
	gvks []schema.GroupVersionKind
}

func (g *recordingGate) Register(_ func(), gvks ...schema.GroupVersionKind) {
	g.gvks = append(g.gvks, gvks...)
}

func TestSetupGatedRegistersAllKinds(t *testing.T) {
	gate := &recordingGate{}

	if err := SetupGated(newManager(t), options(gate)); err != nil {
		t.Fatalf("SetupGated() unexpected error: %v", err)
	}
	want := []schema.GroupVersionKind{
		v1beta1.ClusterProviderConfigGroupVersionKind,
		v1beta1.ProviderConfigGroupVersionKind,
		v1beta1.ProviderConfigUsageGroupVersionKind,
		projectv1alpha1.ProjectGroupVersionKind,
		applicationv1alpha1.ApplicationGroupVersionKind,
		projectconfigurationv1alpha1.ProjectConfigurationGroupVersionKind,
		tenantconfigurationv1alpha1.TenantConfigurationGroupVersionKind,
		accessassignmentv1alpha1.AccessAssignmentGroupVersionKind,
		projectgitsettingsv1alpha1.ProjectGitSettingsGroupVersionKind,
		groupv1alpha1.GroupGroupVersionKind,
	}
	byString := func(a, b schema.GroupVersionKind) int { return cmp.Compare(a.String(), b.String()) }
	slices.SortFunc(gate.gvks, byString)
	slices.SortFunc(want, byString)
	if !slices.Equal(gate.gvks, want) {
		t.Fatalf("gate registered %v, want %v", gate.gvks, want)
	}
}

func TestRunStopsAtFirstError(t *testing.T) {
	boom := errors.New("boom")
	ranAfterError := false

	err := run(nil, options(nil),
		func(ctrl.Manager, controller.Options) error { return boom },
		func(ctrl.Manager, controller.Options) error { ranAfterError = true; return nil },
	)

	if !errors.Is(err, boom) {
		t.Errorf("run() error = %v, want %v", err, boom)
	}
	if ranAfterError {
		t.Error("run() called a setup after one failed")
	}
}
