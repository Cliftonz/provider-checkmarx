// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package projectconfiguration

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/google/uuid"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/Cliftonz/provider-checkmarx/apis/projectconfiguration/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/configuration"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

type external struct {
	client *configuration.ClientWithResponses
}

func newExternal(c *clients.Connection) managed.TypedExternalClient[*v1alpha1.ProjectConfiguration] {
	client, err := configuration.NewClientWithResponses(c.BaseURL, configuration.WithHTTPClient(c.HTTPClient))
	if err != nil {
		panic(err)
	}
	return &external{client: client}
}

func (e *external) Observe(ctx context.Context, cr *v1alpha1.ProjectConfiguration) (managed.ExternalObservation, error) {
	ext := meta.GetExternalName(cr)
	if ext == "" || meta.WasDeleted(cr) {
		return managed.ExternalObservation{}, nil
	}
	id, err := kit.ExternalUUID(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	r, err := e.client.GetProjectConfigurationWithResponse(ctx, &configuration.GetProjectConfigurationParams{ProjectId: id})
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if kit.NotFound(r) {
		return managed.ExternalObservation{}, nil
	}
	if err := kit.CheckStatus(fmt.Sprintf("get configuration of project %q", ext), r); err != nil {
		return managed.ExternalObservation{}, err
	}
	if r.JSON200 == nil {
		return managed.ExternalObservation{}, fmt.Errorf("cannot get configuration of project %q: %s without a configuration body", ext, r.Status())
	}
	remote := map[string]string{}
	for _, entry := range *r.JSON200 {
		remote[entry.Key] = entry.Value
	}
	cr.SetConditions(xpv2.Available())
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: listedKeysMatch(cr.Spec.ForProvider.Entries, remote)}, nil
}

func listedKeysMatch(entries []v1alpha1.ConfigurationEntry, remote map[string]string) bool {
	for _, entry := range entries {
		if value, ok := remote[entry.Key]; !ok || value != entry.Value {
			return false
		}
	}
	return true
}

func (e *external) Create(_ context.Context, cr *v1alpha1.ProjectConfiguration) (managed.ExternalCreation, error) {
	meta.SetExternalName(cr, cr.Spec.ForProvider.ProjectID)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *v1alpha1.ProjectConfiguration) (managed.ExternalUpdate, error) {
	ext := meta.GetExternalName(cr)
	entries := make(configuration.ConfigurationEntries, len(cr.Spec.ForProvider.Entries))
	for i, entry := range cr.Spec.ForProvider.Entries {
		entries[i] = configuration.ConfigurationEntry(entry)
	}
	r, err := e.client.UpdateProjectConfigurationWithResponse(ctx, &configuration.UpdateProjectConfigurationParams{ProjectId: uuid.MustParse(ext)}, entries)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	return managed.ExternalUpdate{}, kit.CheckStatus(fmt.Sprintf("update configuration of project %q", ext), r)
}

func (e *external) Delete(context.Context, *v1alpha1.ProjectConfiguration) (managed.ExternalDelete, error) {
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}
