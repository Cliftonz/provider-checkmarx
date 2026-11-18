// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package accessassignment

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/Cliftonz/provider-checkmarx/apis/accessassignment/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/accessmanagement"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	return kit.Setup(mgr, o, v1alpha1.AccessAssignmentGroupVersionKind, &v1alpha1.AccessAssignment{}, &v1alpha1.AccessAssignmentList{}, newExternal)
}

func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return kit.SetupGated(mgr, o, v1alpha1.AccessAssignmentGroupVersionKind, &v1alpha1.AccessAssignment{}, &v1alpha1.AccessAssignmentList{}, newExternal)
}

func newExternal(c *clients.Connection) managed.TypedExternalClient[*v1alpha1.AccessAssignment] {
	client, err := accessmanagement.NewClientWithResponses(c.BaseURL, accessmanagement.WithHTTPClient(c.HTTPClient))
	if err != nil {
		panic(err)
	}
	return &external{client: client}
}
