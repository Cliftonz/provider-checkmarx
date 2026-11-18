// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package projectgitsettings

import (
	"context"
	"crypto/sha256"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/Cliftonz/provider-checkmarx/apis/projectgitsettings/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/projects"
	"github.com/Cliftonz/provider-checkmarx/internal/controller/kit"
)

type external struct {
	kube   client.Client
	client *projects.ClientWithResponses
}

func (e *external) Observe(ctx context.Context, cr *v1alpha1.ProjectGitSettings) (managed.ExternalObservation, error) {
	name := meta.GetExternalName(cr)
	if name == "" || meta.WasDeleted(cr) {
		return managed.ExternalObservation{}, nil
	}
	id, err := kit.ExternalUUID(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	resp, err := e.client.GetProjectGitSettingsWithResponse(ctx, id)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if kit.NotFound(resp) {
		return managed.ExternalObservation{}, nil
	}
	if err := kit.CheckStatus(fmt.Sprintf("get git settings of project %q", name), resp); err != nil {
		return managed.ExternalObservation{}, err
	}
	remote := resp.JSON200
	if remote == nil {
		return managed.ExternalObservation{}, fmt.Errorf("cannot get git settings of project %q: %s without a body", name, resp.Status())
	}
	_, hash, err := e.readPrivateKey(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	fp := cr.Spec.ForProvider
	cr.SetConditions(xpv2.Available())
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: remote.Url == fp.URL && remote.Branch == fp.Branch && hash == cr.Status.AtProvider.PrivateKeyHash,
	}, nil
}

func (e *external) Create(ctx context.Context, cr *v1alpha1.ProjectGitSettings) (managed.ExternalCreation, error) {
	id, err := parseProjectID(cr.Spec.ForProvider.ProjectID)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	if err := e.applyGitSettings(ctx, cr, id); err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(cr, cr.Spec.ForProvider.ProjectID)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, cr *v1alpha1.ProjectGitSettings) (managed.ExternalUpdate, error) {
	id, err := kit.ExternalUUID(cr)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	return managed.ExternalUpdate{}, e.applyGitSettings(ctx, cr, id)
}

func (e *external) applyGitSettings(ctx context.Context, cr *v1alpha1.ProjectGitSettings, id projects.Id) error {
	key, hash, err := e.readPrivateKey(ctx, cr)
	if err != nil {
		return err
	}
	fp := cr.Spec.ForProvider
	resp, err := e.client.SetProjectGitSettingsWithResponse(ctx, id, projects.GitSettingsInput{Url: fp.URL, Branch: fp.Branch, PrivateKey: &key})
	if err != nil {
		return err
	}
	if err := kit.CheckStatus(fmt.Sprintf("set git settings of project %q", id), resp); err != nil {
		return err
	}
	cr.Status.AtProvider.PrivateKeyHash = hash
	return nil
}

func parseProjectID(s string) (projects.Id, error) {
	var id projects.Id
	if err := id.UnmarshalText([]byte(s)); err != nil {
		return id, fmt.Errorf("project id %q: %w", s, err)
	}
	return id, nil
}

func (e *external) readPrivateKey(ctx context.Context, cr *v1alpha1.ProjectGitSettings) (key, hash string, err error) {
	ref := cr.Spec.ForProvider.PrivateKey
	s := &corev1.Secret{}
	if err = e.kube.Get(ctx, types.NamespacedName{Namespace: cr.GetNamespace(), Name: ref.Name}, s); err != nil {
		return "", "", err
	}
	raw, ok := s.Data[ref.Key]
	if !ok {
		return "", "", fmt.Errorf("key %q is missing from Secret %q", ref.Key, ref.Name)
	}
	return string(raw), fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}

func (e *external) Delete(context.Context, *v1alpha1.ProjectGitSettings) (managed.ExternalDelete, error) {
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}
