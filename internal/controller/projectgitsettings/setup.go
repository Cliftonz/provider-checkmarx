// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package projectgitsettings

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/Cliftonz/provider-checkmarx/apis/projectgitsettings/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/projects"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	return kit.Setup(mgr, o, v1alpha1.ProjectGitSettingsGroupVersionKind, &v1alpha1.ProjectGitSettings{}, &v1alpha1.ProjectGitSettingsList{}, newExternal(mgr))
}

func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return kit.SetupGated(mgr, o, v1alpha1.ProjectGitSettingsGroupVersionKind, &v1alpha1.ProjectGitSettings{}, &v1alpha1.ProjectGitSettingsList{}, newExternal(mgr))
}

func newExternal(mgr ctrl.Manager) kit.NewExternal[*v1alpha1.ProjectGitSettings] {
	return func(conn *clients.Connection) managed.TypedExternalClient[*v1alpha1.ProjectGitSettings] {
		c, err := projects.NewClientWithResponses(conn.BaseURL, projects.WithHTTPClient(conn.HTTPClient))
		if err != nil {
			panic(err)
		}
		return &external{kube: mgr.GetClient(), client: c}
	}
}
