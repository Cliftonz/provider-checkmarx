// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/Cliftonz/provider-checkmarx/internal/clients/clienttest"
)

const projectID = "0b6f3a52-6a0e-4c1e-9d5e-2f7c1a9e8b10"

func TestOperations(t *testing.T) {
	id := uuid.MustParse(projectID)
	entries := []ConfigurationEntry{{Key: "scan.config.sast.presetName", Value: "ASA Premium"}}
	cases := map[string]clienttest.Case[*ClientWithResponses]{
		"getProjectConfiguration": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.GetProjectConfigurationWithResponse(ctx, &GetProjectConfigurationParams{ProjectId: id})
				return err
			},
			Status:   http.StatusOK,
			Response: `[]`,
			Want:     clienttest.Request{Method: http.MethodGet, Path: "/api/configuration/project", Query: url.Values{"project-id": {projectID}}},
		},
		"updateProjectConfiguration": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.UpdateProjectConfigurationWithResponse(ctx, &UpdateProjectConfigurationParams{ProjectId: id}, entries)
				return err
			},
			Status: http.StatusNoContent,
			Want: clienttest.Request{
				Method:   http.MethodPatch,
				Path:     "/api/configuration/project",
				Query:    url.Values{"project-id": {projectID}},
				BodyKeys: []string{"key", "value"},
			},
		},
		"getTenantConfiguration": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.GetTenantConfigurationWithResponse(ctx)
				return err
			},
			Status:   http.StatusOK,
			Response: `[]`,
			Want:     clienttest.Request{Method: http.MethodGet, Path: "/api/configuration/tenant"},
		},
		"updateTenantConfiguration": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.UpdateTenantConfigurationWithResponse(ctx, entries)
				return err
			},
			Status: http.StatusNoContent,
			Want:   clienttest.Request{Method: http.MethodPatch, Path: "/api/configuration/tenant", BodyKeys: []string{"key", "value"}},
		},
	}
	clienttest.Run(t, NewClientWithResponses, recordEntries, cases)
}

func recordEntries(t *testing.T, r *http.Request) clienttest.Request {
	got, raw := clienttest.RecordRaw(t, r)
	if len(raw) == 0 {
		return got
	}
	var body []map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	got.BodyKeys = slices.Sorted(maps.Keys(body[0]))
	return got
}

func TestGeneratedClientStartsWithBoilerplate(t *testing.T) {
	clienttest.AssertGeneratedHeader(t)
}
