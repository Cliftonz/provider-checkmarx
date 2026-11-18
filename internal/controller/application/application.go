// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/uuid"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/Cliftonz/provider-checkmarx/apis/application/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/applications"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

type external struct {
	client *applications.ClientWithResponses
}

func (e *external) Observe(ctx context.Context, cr *v1alpha1.Application) (managed.ExternalObservation, error) {
	name := meta.GetExternalName(cr)
	if name == "" {
		return managed.ExternalObservation{}, nil
	}
	id, err := kit.ExternalUUID(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	resp, err := e.client.GetApplicationWithResponse(ctx, id)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if kit.NotFound(resp) {
		return managed.ExternalObservation{}, nil
	}
	if err := kit.CheckStatus(fmt.Sprintf("get application %q", name), resp); err != nil {
		return managed.ExternalObservation{}, err
	}
	remote := resp.JSON200
	if remote == nil {
		return managed.ExternalObservation{}, fmt.Errorf("cannot get application %q: %s without an application body", name, resp.Status())
	}
	lateInitialized := lateInitialize(&cr.Spec.ForProvider, remote)
	cr.Status.AtProvider = observation(name, remote)
	cr.SetConditions(xpv2.Available())
	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        upToDate(cr.Spec.ForProvider, remote),
		ResourceLateInitialized: lateInitialized,
	}, nil
}

func lateInitialize(fp *v1alpha1.ApplicationParameters, remote *applications.Application) bool {
	initialized := false
	if fp.Description == nil && remote.Description != nil {
		fp.Description = remote.Description
		initialized = true
	}
	if fp.Criticality == nil && remote.Criticality != nil {
		fp.Criticality = remote.Criticality
		initialized = true
	}
	if fp.Type == nil && remote.Type != nil {
		fp.Type = remote.Type
		initialized = true
	}
	if fp.Rules == nil && len(rules(remote)) > 0 {
		fp.Rules = rules(remote)
		initialized = true
	}
	if fp.Tags == nil && len(tags(remote)) > 0 {
		fp.Tags = tags(remote)
		initialized = true
	}
	return initialized
}

func upToDate(fp v1alpha1.ApplicationParameters, remote *applications.Application) bool {
	return fp.Name == kit.Value(remote.Name) &&
		reflect.DeepEqual(fp.Description, remote.Description) &&
		reflect.DeepEqual(fp.Criticality, remote.Criticality) &&
		reflect.DeepEqual(fp.Type, remote.Type) &&
		slices.Equal(fp.Rules, rules(remote)) &&
		maps.Equal(fp.Tags, tags(remote))
}

func rules(remote *applications.Application) []v1alpha1.ApplicationRule {
	var out []v1alpha1.ApplicationRule
	for _, r := range kit.Value(remote.Rules) {
		out = append(out, v1alpha1.ApplicationRule{Type: kit.Value(r.Type), Value: kit.Value(r.Value)})
	}
	return out
}

func tags(remote *applications.Application) map[string]string {
	return kit.Value(remote.Tags)
}

func observation(id string, remote *applications.Application) v1alpha1.ApplicationObservation {
	o := v1alpha1.ApplicationObservation{
		ID:        id,
		CreatedAt: kit.Time(remote.CreatedAt),
		UpdatedAt: kit.Time(remote.UpdatedAt),
	}
	for _, r := range kit.Value(remote.Rules) {
		rule := v1alpha1.ObservedApplicationRule{Type: kit.Value(r.Type), Value: kit.Value(r.Value)}
		if r.Id != nil {
			rule.ID = r.Id.String()
		}
		o.Rules = append(o.Rules, rule)
	}
	for _, id := range kit.Value(remote.ProjectIds) {
		o.ProjectIDs = append(o.ProjectIDs, id.String())
	}
	return o
}

func input(fp v1alpha1.ApplicationParameters) applications.ApplicationInput {
	in := applications.ApplicationInput{
		Name:        fp.Name,
		Description: fp.Description,
		Criticality: fp.Criticality,
		Type:        fp.Type,
	}
	if len(fp.Rules) > 0 {
		rs := make([]applications.ApplicationRule, 0, len(fp.Rules))
		for _, r := range fp.Rules {
			rs = append(rs, applications.ApplicationRule{Type: &r.Type, Value: &r.Value})
		}
		in.Rules = &rs
	}
	if len(fp.Tags) > 0 {
		in.Tags = &fp.Tags
	}
	return in
}

func (e *external) Create(ctx context.Context, cr *v1alpha1.Application) (managed.ExternalCreation, error) {
	fp := cr.Spec.ForProvider
	resp, err := e.client.CreateApplicationWithResponse(ctx, input(fp))
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	if err := kit.CheckStatus(fmt.Sprintf("create application %q", fp.Name), resp); err != nil {
		return managed.ExternalCreation{}, err
	}
	created := resp.JSON201
	if created == nil {
		return managed.ExternalCreation{}, fmt.Errorf("cannot create application %q: %s without an application body", fp.Name, resp.Status())
	}
	if created.Id == nil {
		return managed.ExternalCreation{}, fmt.Errorf("cannot create application %q: response without an id", fp.Name)
	}
	meta.SetExternalName(cr, created.Id.String())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *v1alpha1.Application) (managed.ExternalUpdate, error) {
	name := meta.GetExternalName(cr)
	resp, err := e.client.UpdateApplicationWithResponse(ctx, uuid.MustParse(name), input(cr.Spec.ForProvider))
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	return managed.ExternalUpdate{}, kit.CheckStatus(fmt.Sprintf("update application %q", name), resp)
}

func (e *external) Delete(ctx context.Context, cr *v1alpha1.Application) (managed.ExternalDelete, error) {
	name := meta.GetExternalName(cr)
	resp, err := e.client.DeleteApplicationWithResponse(ctx, uuid.MustParse(name))
	if err != nil {
		return managed.ExternalDelete{}, err
	}
	if kit.NotFound(resp) {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, kit.CheckStatus(fmt.Sprintf("delete application %q", name), resp)
}

func (e *external) Disconnect(context.Context) error {
	return nil
}
