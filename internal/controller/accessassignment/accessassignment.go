// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package accessassignment

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/Cliftonz/provider-checkmarx/apis/accessassignment/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/accessmanagement"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

type external struct {
	client *accessmanagement.ClientWithResponses
}

func (e *external) Observe(ctx context.Context, cr *v1alpha1.AccessAssignment) (managed.ExternalObservation, error) {
	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{}, nil
	}
	fp := cr.Spec.ForProvider
	resp, err := e.client.ListAssignmentsWithResponse(ctx, &accessmanagement.ListAssignmentsParams{
		ResourceId:   uuid.MustParse(fp.ResourceID),
		ResourceType: fp.ResourceType,
	})
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if kit.NotFound(resp) {
		return managed.ExternalObservation{}, nil
	}
	if err := kit.CheckStatus(fmt.Sprintf("list assignments for %s %q", fp.ResourceType, fp.ResourceID), resp); err != nil {
		return managed.ExternalObservation{}, err
	}
	if resp.JSON200 == nil {
		return managed.ExternalObservation{}, fmt.Errorf("cannot list assignments for %s %q: %s without an assignment list", fp.ResourceType, fp.ResourceID, resp.Status())
	}
	i := slices.IndexFunc(*resp.JSON200, func(a accessmanagement.AssignmentResponse) bool {
		return kit.Value(a.EntityID) == uuid.MustParse(fp.EntityID) && kit.Value(a.EntityType) == fp.EntityType
	})
	if i < 0 {
		return managed.ExternalObservation{}, nil
	}
	cr.SetConditions(xpv2.Available())
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: slices.Equal(roleNames(fp.EntityRoles, func(r v1alpha1.Role) string { return r.Name }), roleNames(kit.Value((*resp.JSON200)[i].EntityRoles), func(r accessmanagement.Role) string { return r.Name })),
	}, nil
}

func roleNames[R any](roles []R, name func(R) string) []string {
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, name(r))
	}
	return slices.Compact(slices.Sorted(slices.Values(names)))
}

func apiRoles(roles []v1alpha1.Role) []accessmanagement.Role {
	out := make([]accessmanagement.Role, 0, len(roles))
	for _, r := range roles {
		role := accessmanagement.Role{Name: r.Name}
		if r.ID != "" {
			id := uuid.MustParse(r.ID)
			role.Id = &id
		}
		out = append(out, role)
	}
	return out
}

func (e *external) Create(ctx context.Context, cr *v1alpha1.AccessAssignment) (managed.ExternalCreation, error) {
	fp := cr.Spec.ForProvider
	resp, err := e.client.CreateAssignmentWithResponse(ctx, accessmanagement.AssignmentInput{
		EntityID:     uuid.MustParse(fp.EntityID),
		EntityType:   fp.EntityType,
		EntityRoles:  apiRoles(fp.EntityRoles),
		ResourceID:   uuid.MustParse(fp.ResourceID),
		ResourceType: fp.ResourceType,
	})
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	if err := kit.CheckStatus(fmt.Sprintf("create assignment of %s %q on %s %q", fp.EntityType, fp.EntityID, fp.ResourceType, fp.ResourceID), resp); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(cr, fp.EntityID+"/"+fp.ResourceID)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *v1alpha1.AccessAssignment) (managed.ExternalUpdate, error) {
	if _, err := e.Delete(ctx, cr); err != nil {
		return managed.ExternalUpdate{}, err
	}
	_, err := e.Create(ctx, cr)
	return managed.ExternalUpdate{}, err
}

func (e *external) Delete(ctx context.Context, cr *v1alpha1.AccessAssignment) (managed.ExternalDelete, error) {
	fp := cr.Spec.ForProvider
	resp, err := e.client.DeleteAssignmentWithResponse(ctx, &accessmanagement.DeleteAssignmentParams{
		EntityId:   uuid.MustParse(fp.EntityID),
		ResourceId: uuid.MustParse(fp.ResourceID),
	})
	if err != nil {
		return managed.ExternalDelete{}, err
	}
	if kit.NotFound(resp) {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, kit.CheckStatus(fmt.Sprintf("delete assignment of %s %q on %s %q", fp.EntityType, fp.EntityID, fp.ResourceType, fp.ResourceID), resp)
}

func (e *external) Disconnect(context.Context) error {
	return nil
}
