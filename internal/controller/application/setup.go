// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/Cliftonz/provider-checkmarx/apis/application/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/applications"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	return kit.Setup(mgr, o, v1alpha1.ApplicationGroupVersionKind, &v1alpha1.Application{}, &v1alpha1.ApplicationList{}, newExternal)
}

func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return kit.SetupGated(mgr, o, v1alpha1.ApplicationGroupVersionKind, &v1alpha1.Application{}, &v1alpha1.ApplicationList{}, newExternal)
}

func newExternal(c *clients.Connection) managed.TypedExternalClient[*v1alpha1.Application] {
	client, err := applications.NewClientWithResponses(c.BaseURL, applications.WithHTTPClient(c.HTTPClient))
	if err != nil {
		panic(err)
	}
	return &external{client: client}
}
