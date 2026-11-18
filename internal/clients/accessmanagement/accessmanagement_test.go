// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package accessmanagement

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"

	"github.com/Cliftonz/provider-checkmarx/internal/clients/clienttest"
)

const (
	groupID   = "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d"
	projectID = "0b6f3a52-6a0e-4c1e-9d5e-2f7c1a9e8b10"
	roleID    = "1f2e3d4c-5b6a-4978-8a6b-5c4d3e2f1a0b"
)

const assignment = `{"entityID":"` + groupID + `","entityType":"group","entityName":"devs",` +
	`"entityRoles":[{"id":"` + roleID + `","name":"view-results"}],` +
	`"resourceID":"` + projectID + `","resourceType":"project","resourceName":"app"}`

func TestOperations(t *testing.T) {
	entity := uuid.MustParse(groupID)
	resource := uuid.MustParse(projectID)
	cases := map[string]clienttest.Case[*ClientWithResponses]{
		"createAssignment": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				resp, err := c.CreateAssignmentWithResponse(ctx, AssignmentInput{
					EntityID:     entity,
					EntityType:   "group",
					EntityRoles:  []Role{{Id: new(uuid.MustParse(roleID)), Name: "view-results"}},
					ResourceType: "project",
					ResourceID:   resource,
				})
				if err != nil {
					return err
				}
				return assertAssignment(resp.JSON201)
			},
			Status:   http.StatusCreated,
			Response: assignment,
			Want: clienttest.Request{
				Method:   http.MethodPost,
				Path:     "/api/access-management",
				BodyKeys: []string{"entityID", "entityRoles", "entityType", "resourceID", "resourceType"},
			},
		},
		"listAssignments": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				resp, err := c.ListAssignmentsWithResponse(ctx, &ListAssignmentsParams{ResourceId: resource, ResourceType: "project"})
				if err != nil {
					return err
				}
				return assertAssignment(&(*resp.JSON200)[0])
			},
			Status:   http.StatusOK,
			Response: "[" + assignment + "]",
			Want: clienttest.Request{
				Method: http.MethodGet,
				Path:   "/api/access-management/entities-for",
				Query:  url.Values{"resource-id": {projectID}, "resource-type": {"project"}},
			},
		},
		"deleteAssignment": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.DeleteAssignmentWithResponse(ctx, &DeleteAssignmentParams{EntityId: entity, ResourceId: resource})
				return err
			},
			Status: http.StatusNoContent,
			Want: clienttest.Request{
				Method: http.MethodDelete,
				Path:   "/api/access-management",
				Query:  url.Values{"entity-id": {groupID}, "resource-id": {projectID}},
			},
		},
	}
	clienttest.Run(t, NewClientWithResponses, clienttest.RecordObject, cases)
}

func assertAssignment(a *AssignmentResponse) error {
	if *a.EntityName != "devs" || *a.ResourceName != "app" || (*a.EntityRoles)[0].Name != "view-results" {
		return fmt.Errorf("assignment = %+v", *a)
	}
	return nil
}

func TestGeneratedClientStartsWithBoilerplate(t *testing.T) {
	clienttest.AssertGeneratedHeader(t)
}
