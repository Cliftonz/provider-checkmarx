// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package tenantconfiguration

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/Cliftonz/provider-checkmarx/apis/tenantconfiguration/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/configuration"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

func Setup(mgr ctrl.Manager, o controller.Options) error {
	return kit.Setup(mgr, o, v1alpha1.TenantConfigurationGroupVersionKind, &v1alpha1.TenantConfiguration{}, &v1alpha1.TenantConfigurationList{}, newExternal)
}

func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return kit.SetupGated(mgr, o, v1alpha1.TenantConfigurationGroupVersionKind, &v1alpha1.TenantConfiguration{}, &v1alpha1.TenantConfigurationList{}, newExternal)
}

func newExternal(c *clients.Connection) managed.TypedExternalClient[*v1alpha1.TenantConfiguration] {
	client, err := configuration.NewClientWithResponses(c.BaseURL, configuration.WithHTTPClient(c.HTTPClient))
	if err != nil {
		panic(err)
	}
	return &external{client: client}
}
