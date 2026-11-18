// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package tenantconfiguration

import (
	"context"
	"fmt"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/Cliftonz/provider-checkmarx/apis/tenantconfiguration/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/configuration"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

const externalName = "tenant"

type external struct {
	client *configuration.ClientWithResponses
}

func (e *external) Observe(ctx context.Context, cr *v1alpha1.TenantConfiguration) (managed.ExternalObservation, error) {
	if meta.GetExternalName(cr) == "" || meta.WasDeleted(cr) {
		return managed.ExternalObservation{}, nil
	}
	resp, err := e.client.GetTenantConfigurationWithResponse(ctx)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if kit.NotFound(resp) {
		return managed.ExternalObservation{}, nil
	}
	if err := kit.CheckStatus("get tenant configuration", resp); err != nil {
		return managed.ExternalObservation{}, err
	}
	if resp.JSON200 == nil {
		return managed.ExternalObservation{}, fmt.Errorf("cannot get tenant configuration: unexpected %s without a body", resp.Status())
	}
	remote := map[string]string{}
	for _, entry := range *resp.JSON200 {
		remote[entry.Key] = entry.Value
	}
	upToDate := true
	for _, entry := range cr.Spec.ForProvider.Entries {
		value, ok := remote[entry.Key]
		upToDate = upToDate && ok && value == entry.Value
	}
	cr.SetConditions(xpv2.Available())
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
}

func (e *external) Create(_ context.Context, cr *v1alpha1.TenantConfiguration) (managed.ExternalCreation, error) {
	meta.SetExternalName(cr, externalName)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *v1alpha1.TenantConfiguration) (managed.ExternalUpdate, error) {
	var entries configuration.ConfigurationEntries
	for _, entry := range cr.Spec.ForProvider.Entries {
		entries = append(entries, configuration.ConfigurationEntry(entry))
	}
	resp, err := e.client.UpdateTenantConfigurationWithResponse(ctx, entries)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	return managed.ExternalUpdate{}, kit.CheckStatus("update tenant configuration", resp)
}

func (e *external) Delete(context.Context, *v1alpha1.TenantConfiguration) (managed.ExternalDelete, error) {
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}
