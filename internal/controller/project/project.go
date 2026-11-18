// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/uuid"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/Cliftonz/provider-checkmarx/apis/project/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/projects"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

type external struct {
	client *projects.ClientWithResponses
}

func (e *external) Observe(ctx context.Context, cr *v1alpha1.Project) (managed.ExternalObservation, error) {
	name := meta.GetExternalName(cr)
	if name == "" {
		return managed.ExternalObservation{}, nil
	}
	id, err := kit.ExternalUUID(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	resp, err := e.client.GetProjectWithResponse(ctx, id)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if kit.NotFound(resp) {
		return managed.ExternalObservation{}, nil
	}
	if err := kit.CheckStatus(fmt.Sprintf("get project %q", name), resp); err != nil {
		return managed.ExternalObservation{}, err
	}
	remote := resp.JSON200
	if remote == nil {
		return managed.ExternalObservation{}, fmt.Errorf("cannot get project %q: %s without a project body", name, resp.Status())
	}
	observed := parameters(remote)
	lateInitialized := lateInitialize(&cr.Spec.ForProvider, observed)
	cr.Status.AtProvider = v1alpha1.ProjectObservation{
		ID:             name,
		CreatedAt:      kit.Time(remote.CreatedAt),
		UpdatedAt:      kit.Time(remote.UpdatedAt),
		ApplicationIDs: ids(remote.ApplicationIds),
	}
	cr.SetConditions(xpv2.Available())
	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        upToDate(cr.Spec.ForProvider, observed),
		ResourceLateInitialized: lateInitialized,
	}, nil
}

func parameters(remote *projects.Project) v1alpha1.ProjectParameters {
	p := v1alpha1.ProjectParameters{
		Name:        kit.Value(remote.Name),
		RepoURL:     remote.RepoUrl,
		MainBranch:  remote.MainBranch,
		Origin:      remote.Origin,
		ScmRepoID:   remote.ScmRepoId,
		Groups:      ids(remote.Groups),
		Criticality: remote.Criticality,
	}
	if len(kit.Value(remote.Tags)) > 0 {
		p.Tags = *remote.Tags
	}
	return p
}

func lateInitialize(fp *v1alpha1.ProjectParameters, remote v1alpha1.ProjectParameters) bool {
	before := fp.DeepCopy()
	if fp.RepoURL == nil {
		fp.RepoURL = remote.RepoURL
	}
	if fp.MainBranch == nil {
		fp.MainBranch = remote.MainBranch
	}
	if fp.Origin == nil {
		fp.Origin = remote.Origin
	}
	if fp.ScmRepoID == nil {
		fp.ScmRepoID = remote.ScmRepoID
	}
	if fp.Tags == nil {
		fp.Tags = remote.Tags
	}
	if fp.Groups == nil {
		fp.Groups = remote.Groups
	}
	if fp.Criticality == nil {
		fp.Criticality = remote.Criticality
	}
	return !reflect.DeepEqual(before, fp)
}

func upToDate(fp, remote v1alpha1.ProjectParameters) bool {
	fp.GroupsRef, fp.GroupsSelector, fp.PrivatePackage = nil, nil, nil
	if len(fp.Tags) == 0 {
		fp.Tags = nil
	}
	fp.Groups, remote.Groups = slices.Sorted(slices.Values(fp.Groups)), slices.Sorted(slices.Values(remote.Groups))
	return reflect.DeepEqual(fp, remote)
}

func ids(remote *[]uuid.UUID) []string {
	var s []string
	for _, id := range kit.Value(remote) {
		s = append(s, id.String())
	}
	return s
}

func input(fp v1alpha1.ProjectParameters) projects.ProjectInput {
	in := projects.ProjectInput{
		Name:           fp.Name,
		RepoUrl:        fp.RepoURL,
		MainBranch:     fp.MainBranch,
		Origin:         fp.Origin,
		ScmRepoId:      fp.ScmRepoID,
		Criticality:    fp.Criticality,
		PrivatePackage: fp.PrivatePackage,
	}
	if len(fp.Tags) > 0 {
		in.Tags = &fp.Tags
	}
	if len(fp.Groups) > 0 {
		groups := make([]uuid.UUID, len(fp.Groups))
		for i, g := range fp.Groups {
			groups[i] = uuid.MustParse(g)
		}
		in.Groups = &groups
	}
	return in
}

func (e *external) Create(ctx context.Context, cr *v1alpha1.Project) (managed.ExternalCreation, error) {
	fp := cr.Spec.ForProvider
	resp, err := e.client.CreateProjectWithResponse(ctx, input(fp))
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	if err := kit.CheckStatus(fmt.Sprintf("create project %q", fp.Name), resp); err != nil {
		return managed.ExternalCreation{}, err
	}
	created := resp.JSON201
	if created == nil {
		return managed.ExternalCreation{}, fmt.Errorf("cannot create project %q: %s without a created project body", fp.Name, resp.Status())
	}
	if created.Id == nil {
		return managed.ExternalCreation{}, fmt.Errorf("cannot create project %q: response has no id", fp.Name)
	}
	meta.SetExternalName(cr, created.Id.String())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *v1alpha1.Project) (managed.ExternalUpdate, error) {
	name := meta.GetExternalName(cr)
	resp, err := e.client.UpdateProjectWithResponse(ctx, uuid.MustParse(name), input(cr.Spec.ForProvider))
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	return managed.ExternalUpdate{}, kit.CheckStatus(fmt.Sprintf("update project %q", name), resp)
}

func (e *external) Delete(ctx context.Context, cr *v1alpha1.Project) (managed.ExternalDelete, error) {
	name := meta.GetExternalName(cr)
	resp, err := e.client.DeleteProjectWithResponse(ctx, uuid.MustParse(name))
	if err != nil {
		return managed.ExternalDelete{}, err
	}
	if kit.NotFound(resp) {
		return managed.ExternalDelete{}, nil
	}
	return managed.ExternalDelete{}, kit.CheckStatus(fmt.Sprintf("delete project %q", name), resp)
}

func (e *external) Disconnect(context.Context) error {
	return nil
}
