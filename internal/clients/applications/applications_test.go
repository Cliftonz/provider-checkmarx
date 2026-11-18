// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package applications

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/Cliftonz/provider-checkmarx/internal/clients/clienttest"
)

const applicationID = "3d2c9f4e-1b7a-4e0f-8c6d-5a4b3c2d1e0f"

func TestOperations(t *testing.T) {
	id := uuid.MustParse(applicationID)
	cases := map[string]clienttest.Case[*ClientWithResponses]{
		"createApplication": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.CreateApplicationWithResponse(ctx, ApplicationInput{
					Name:        "shop",
					Description: new("storefront"),
					Type:        new("web"),
					Criticality: new(3),
					Rules:       &[]ApplicationRule{{Id: new(uuid.MustParse("7e8f9a0b-1c2d-4e3f-8a5b-6c7d8e9f0a1b")), Type: new("project.name.in"), Value: new("app")}},
					Tags:        &map[string]string{"team": "a"},
				})
				return err
			},
			Status:   http.StatusCreated,
			Response: `{}`,
			Want: clienttest.Request{
				Method:   http.MethodPost,
				Path:     "/api/applications",
				BodyKeys: []string{"criticality", "description", "name", "rules", "tags", "type"},
			},
		},
		"getApplication": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.GetApplicationWithResponse(ctx, id)
				return err
			},
			Status:   http.StatusOK,
			Response: `{}`,
			Want:     clienttest.Request{Method: http.MethodGet, Path: "/api/applications/" + applicationID},
		},
		"updateApplication": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.UpdateApplicationWithResponse(ctx, id, ApplicationInput{Name: "shop", Criticality: new(5)})
				return err
			},
			Status: http.StatusNoContent,
			Want:   clienttest.Request{Method: http.MethodPut, Path: "/api/applications/" + applicationID, BodyKeys: []string{"criticality", "name"}},
		},
		"deleteApplication": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.DeleteApplicationWithResponse(ctx, id)
				return err
			},
			Status: http.StatusNoContent,
			Want:   clienttest.Request{Method: http.MethodDelete, Path: "/api/applications/" + applicationID},
		},
	}
	clienttest.Run(t, NewClientWithResponses, clienttest.RecordObject, cases)
}

func TestGeneratedClientStartsWithBoilerplate(t *testing.T) {
	clienttest.AssertGeneratedHeader(t)
}
