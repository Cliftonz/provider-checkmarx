// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package iamgroups

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/Cliftonz/provider-checkmarx/internal/clients/clienttest"
)

const groupID = "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d"

func TestOperations(t *testing.T) {
	id := uuid.MustParse(groupID)
	cases := map[string]clienttest.Case[*ClientWithResponses]{
		"createGroup": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.CreateGroupWithResponse(ctx, "acme", Group{Name: "devs"})
				return err
			},
			Status: http.StatusCreated,
			Want:   clienttest.Request{Method: http.MethodPost, Path: "/auth/admin/realms/acme/groups", BodyKeys: []string{"name"}},
		},
		"getGroup": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.GetGroupWithResponse(ctx, "acme", id)
				return err
			},
			Status:   http.StatusOK,
			Response: `{}`,
			Want:     clienttest.Request{Method: http.MethodGet, Path: "/auth/admin/realms/acme/groups/" + groupID},
		},
		"updateGroup": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.UpdateGroupWithResponse(ctx, "acme", id, Group{Id: &id, Name: "devs"})
				return err
			},
			Status: http.StatusNoContent,
			Want:   clienttest.Request{Method: http.MethodPut, Path: "/auth/admin/realms/acme/groups/" + groupID, BodyKeys: []string{"id", "name"}},
		},
		"deleteGroup": {
			Call: func(ctx context.Context, c *ClientWithResponses) error {
				_, err := c.DeleteGroupWithResponse(ctx, "acme", id)
				return err
			},
			Status: http.StatusNoContent,
			Want:   clienttest.Request{Method: http.MethodDelete, Path: "/auth/admin/realms/acme/groups/" + groupID},
		},
	}
	clienttest.Run(t, NewClientWithResponses, clienttest.RecordObject, cases)
}

func TestGeneratedClientStartsWithBoilerplate(t *testing.T) {
	clienttest.AssertGeneratedHeader(t)
}
