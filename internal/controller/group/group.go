// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"fmt"
	"net/url"
	"path"

	"github.com/google/uuid"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/Cliftonz/provider-checkmarx/apis/group/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/iamgroups"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

type external struct {
	client *iamgroups.ClientWithResponses
	tenant string
}

func (e *external) Observe(ctx context.Context, cr *v1alpha1.Group) (managed.ExternalObservation, error) {
	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{}, nil
	}
	id, err := kit.ExternalUUID(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	resp, err := e.client.GetGroupWithResponse(ctx, e.tenant, id)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if kit.NotFound(resp) {
		return managed.ExternalObservation{}, nil
	}
	if err := kit.CheckStatus(fmt.Sprintf("get group %q", id), resp); err != nil {
		return managed.ExternalObservation{}, err
	}
	if resp.JSON200 == nil {
		return managed.ExternalObservation{}, fmt.Errorf("cannot get group %q: %s without a group body", id, resp.Status())
	}
	cr.Status.AtProvider = v1alpha1.GroupObservation{ID: id.String()}
	cr.SetConditions(xpv2.Available())
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: resp.JSON200.Name == cr.Spec.ForProvider.Name,
	}, nil
}

func (e *external) Create(ctx context.Context, cr *v1alpha1.Group) (managed.ExternalCreation, error) {
	name := cr.Spec.ForProvider.Name
	resp, err := e.client.CreateGroupWithResponse(ctx, e.tenant, iamgroups.Group{Name: name})
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	if err := kit.CheckStatus(fmt.Sprintf("create group %q", name), resp); err != nil {
		return managed.ExternalCreation{}, err
	}
	location := resp.HTTPResponse.Header.Get("Location")
	id, err := idFromLocation(location)
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("cannot read the id of created group %q from Location header %q: %w", name, location, err)
	}
	meta.SetExternalName(cr, id.String())
	return managed.ExternalCreation{}, nil
}

func idFromLocation(location string) (uuid.UUID, error) {
	u, err := url.Parse(location)
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(path.Base(u.Path))
}

func (e *external) Update(ctx context.Context, cr *v1alpha1.Group) (managed.ExternalUpdate, error) {
	id := uuid.MustParse(meta.GetExternalName(cr))
	resp, err := e.client.UpdateGroupWithResponse(ctx, e.tenant, id, iamgroups.Group{Name: cr.Spec.ForProvider.Name})
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	return managed.ExternalUpdate{}, kit.CheckStatus(fmt.Sprintf("update group %q", id), resp)
}

func (e *external) Delete(ctx context.Context, cr *v1alpha1.Group) (managed.ExternalDelete, error) {
	id := uuid.MustParse(meta.GetExternalName(cr))
	resp, err := e.client.DeleteGroupWithResponse(ctx, e.tenant, id)
	if err != nil {
		return managed.ExternalDelete{}, err
	}
	if kit.NotFound(resp) {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, kit.CheckStatus(fmt.Sprintf("delete group %q", id), resp)
}

func (e *external) Disconnect(context.Context) error {
	return nil
}
