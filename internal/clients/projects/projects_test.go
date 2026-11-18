// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package projects

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/Cliftonz/provider-checkmarx/internal/clients/clienttest"
)

const (
	projectID = "0b6f3a52-6a0e-4c1e-9d5e-2f7c1a9e8b10"
	groupID   = "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d"
)

func TestOperations(t *testing.T) {
	id := uuid.MustParse(projectID)
	cases := map[string]clienttest.Case[*ClientWithResponses]{
		"createProject": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.CreateProjectWithResponse(ctx, ProjectInput{
					Name:           "app",
					RepoUrl:        new("https://git.example/app.git"),
					MainBranch:     new("main"),
					Origin:         new("crossplane"),
					ScmRepoId:      new("42"),
					Tags:           &map[string]string{"team": "a"},
					Groups:         &[]uuid.UUID{uuid.MustParse(groupID)},
					Criticality:    new(3),
					PrivatePackage: new(true),
				})
				return err
			},
			Status:   http.StatusCreated,
			Response: `{}`,
			Want: clienttest.Request{
				Method:   http.MethodPost,
				Path:     "/api/projects",
				BodyKeys: []string{"criticality", "groups", "mainBranch", "name", "origin", "privatePackage", "repoUrl", "scmRepoId", "tags"},
			},
		},
		"getProject": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.GetProjectWithResponse(ctx, id)
				return err
			},
			Status:   http.StatusOK,
			Response: `{}`,
			Want:     clienttest.Request{Method: http.MethodGet, Path: "/api/projects/" + projectID},
		},
		"updateProject": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.UpdateProjectWithResponse(ctx, id, ProjectInput{Name: "app", MainBranch: new("dev")})
				return err
			},
			Status: http.StatusNoContent,
			Want:   clienttest.Request{Method: http.MethodPut, Path: "/api/projects/" + projectID, BodyKeys: []string{"mainBranch", "name"}},
		},
		"deleteProject": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.DeleteProjectWithResponse(ctx, id)
				return err
			},
			Status: http.StatusNoContent,
			Want:   clienttest.Request{Method: http.MethodDelete, Path: "/api/projects/" + projectID},
		},
		"getProjectGitSettings": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.GetProjectGitSettingsWithResponse(ctx, id)
				return err
			},
			Status:   http.StatusOK,
			Response: `{}`,
			Want:     clienttest.Request{Method: http.MethodGet, Path: "/api/projects/" + projectID + "/sourceCode/remoteSettings/git"},
		},
		"setProjectGitSettings": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.SetProjectGitSettingsWithResponse(ctx, id, GitSettingsInput{
					Url:        "git@git.example:app.git",
					Branch:     "main",
					PrivateKey: new("key"),
				})
				return err
			},
			Status: http.StatusNoContent,
			Want: clienttest.Request{
				Method:   http.MethodPost,
				Path:     "/api/projects/" + projectID + "/sourceCode/remoteSettings/git",
				BodyKeys: []string{"branch", "privateKey", "url"},
			},
		},
	}
	clienttest.Run(t, NewClientWithResponses, clienttest.RecordObject, cases)
}

func TestGeneratedClientStartsWithBoilerplate(t *testing.T) {
	clienttest.AssertGeneratedHeader(t)
}
