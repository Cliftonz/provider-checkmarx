// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/Cliftonz/provider-checkmarx/apis/group/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/iamgroups"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	return kit.Setup(mgr, o, v1alpha1.GroupGroupVersionKind, &v1alpha1.Group{}, &v1alpha1.GroupList{}, newExternal)
}

func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return kit.SetupGated(mgr, o, v1alpha1.GroupGroupVersionKind, &v1alpha1.Group{}, &v1alpha1.GroupList{}, newExternal)
}

func newExternal(conn *clients.Connection) managed.TypedExternalClient[*v1alpha1.Group] {
	c, err := iamgroups.NewClientWithResponses(conn.IAMURL, iamgroups.WithHTTPClient(conn.HTTPClient))
	if err != nil {
		panic(err)
	}
	return &external{client: c, tenant: conn.Tenant}
}
