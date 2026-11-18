// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/Cliftonz/provider-checkmarx/apis/project/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/projects"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	return kit.Setup(mgr, o, v1alpha1.ProjectGroupVersionKind, &v1alpha1.Project{}, &v1alpha1.ProjectList{}, newExternal)
}

func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return kit.SetupGated(mgr, o, v1alpha1.ProjectGroupVersionKind, &v1alpha1.Project{}, &v1alpha1.ProjectList{}, newExternal)
}

func newExternal(c *clients.Connection) managed.TypedExternalClient[*v1alpha1.Project] {
	client, err := projects.NewClientWithResponses(c.BaseURL, projects.WithHTTPClient(c.HTTPClient))
	if err != nil {
		panic(err)
	}
	return &external{client: client}
}
