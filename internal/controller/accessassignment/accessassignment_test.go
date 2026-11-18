// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package accessassignment

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/Cliftonz/provider-checkmarx/apis/accessassignment/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
)

const (
	entityID     = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
	otherEntity  = "9b2e8f10-1c4d-4e5a-8f6b-7c8d9e0f1a2b"
	resourceID   = "c56a4180-65aa-42ec-a945-5fd21dec0538"
	scannerRole  = "1d1c8f3e-2b4a-4c5d-9e6f-7a8b9c0d1e2f"
	viewerRole   = "2e2d9f4f-3c5b-4d6e-8f7a-8b9c0d1e2f3a"
	adminRole    = "4f3e0a5b-6d7c-4e8f-9a0b-1c2d3e4f5a6b"
	externalName = entityID + "/" + resourceID
	listPath     = "/api/access-management/entities-for"
	accessPath   = "/api/access-management"
	listQuery    = "resource-id=" + resourceID + "&resource-type=project"
	deleteQuery  = "entity-id=" + entityID + "&resource-id=" + resourceID
)

type request struct {
	Method string
	Path   string
	Query  string
	Body   map[string]any
}

type fakeAPI struct {
	status   int
	response string
	received []request
}

func connectTo(t *testing.T, api *fakeAPI) managed.TypedExternalClient[*v1alpha1.AccessAssignment] {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		api.received = append(api.received, request{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(api.status)
		_, _ = io.WriteString(w, api.response)
	}))
	t.Cleanup(srv.Close)
	return newExternal(&clients.Connection{HTTPClient: srv.Client(), BaseURL: srv.URL})
}

func assignment(externalName string, roles ...v1alpha1.Role) *v1alpha1.AccessAssignment {
	cr := &v1alpha1.AccessAssignment{Spec: v1alpha1.AccessAssignmentSpec{ForProvider: v1alpha1.AccessAssignmentParameters{
		EntityID:     entityID,
		EntityType:   "group",
		EntityRoles:  roles,
		ResourceType: "project",
		ResourceID:   resourceID,
	}}}
	meta.SetExternalName(cr, externalName)
	return cr
}

func assertReceived(t *testing.T, api *fakeAPI, want ...request) {
	t.Helper()
	if !reflect.DeepEqual(api.received, want) {
		t.Errorf("received requests\n got: %#v\nwant: %#v", api.received, want)
	}
}

var (
	scanner = v1alpha1.Role{ID: scannerRole, Name: "ast-scanner"}
	viewer  = v1alpha1.Role{ID: viewerRole, Name: "ast-viewer"}
	listGet = request{Method: http.MethodGet, Path: listPath, Query: listQuery}
	del     = request{Method: http.MethodDelete, Path: accessPath, Query: deleteQuery}
	post    = request{Method: http.MethodPost, Path: accessPath, Body: map[string]any{
		"entityID":   entityID,
		"entityType": "group",
		"entityRoles": []any{
			map[string]any{"id": scannerRole, "name": "ast-scanner"},
			map[string]any{"name": "ast-viewer"},
		},
		"resourceID":   resourceID,
		"resourceType": "project",
	}}
)

func TestObserveWithoutExternalNameDoesNotExist(t *testing.T) {
	api := &fakeAPI{}
	got, err := connectTo(t, api).Observe(t.Context(), assignment("", scanner))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
	assertReceived(t, api)
}

func TestObserveWithoutMatchingEntryDoesNotExist(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `[
		{"entityID":"` + entityID + `","entityType":"user","entityRoles":[{"name":"ast-scanner"}],"resourceID":"` + resourceID + `","resourceType":"project"},
		{"entityID":"` + otherEntity + `","entityType":"group","entityRoles":[{"name":"ast-scanner"}],"resourceID":"` + resourceID + `","resourceType":"project"}
	]`}
	got, err := connectTo(t, api).Observe(t.Context(), assignment(externalName, scanner))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
	assertReceived(t, api, listGet)
}

func TestObserveNotFoundDoesNotExist(t *testing.T) {
	api := &fakeAPI{status: http.StatusNotFound, response: `{"message":"not found"}`}
	got, err := connectTo(t, api).Observe(t.Context(), assignment(externalName, scanner))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
}

func TestObserveServerErrorIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusInternalServerError, response: `{"message":"boom"}`}
	_, err := connectTo(t, api).Observe(t.Context(), assignment(externalName, scanner))
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %v, want an error reporting HTTP 500", err)
	}
}

func TestObserveRolesMatchingByNameInAnyOrderIsUpToDate(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `[
		{"entityID":"` + entityID + `","entityType":"group","entityName":"devs","entityRoles":[{"id":"` + adminRole + `","name":"ast-viewer"},{"name":"ast-scanner"}],"resourceID":"` + resourceID + `","resourceType":"project","resourceName":"web"}
	]`}
	cr := assignment(externalName, scanner, viewer)
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	if want := (managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
	if c := cr.GetCondition(xpv2.TypeReady); c.Reason != xpv2.ReasonAvailable {
		t.Errorf("ready condition = %+v, want Available", c)
	}
	assertReceived(t, api, listGet)
}

func TestObserveRolesDifferingByNameIsDrift(t *testing.T) {
	cases := map[string]string{
		"Missing": `[{"name":"ast-scanner"}]`,
		"Extra":   `[{"name":"ast-scanner"},{"name":"ast-viewer"},{"name":"ast-admin"}]`,
		"Renamed": `[{"name":"ast-scanner"},{"name":"ast-reviewer"}]`,
		"Absent":  `null`,
	}
	for name, roles := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{status: http.StatusOK, response: `[{"entityID":"` + entityID + `","entityType":"group","entityRoles":` + roles + `,"resourceID":"` + resourceID + `","resourceType":"project"}]`}
			got, err := connectTo(t, api).Observe(t.Context(), assignment(externalName, scanner, viewer))
			if err != nil {
				t.Fatal(err)
			}
			if want := (managed.ExternalObservation{ResourceExists: true}); !reflect.DeepEqual(got, want) {
				t.Errorf("observation = %+v, want %+v", got, want)
			}
		})
	}
}

func TestCreatePostsAssignmentAndSetsExternalName(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated, response: `{"entityID":"` + entityID + `","entityType":"group","entityRoles":[{"name":"ast-scanner"}],"resourceID":"` + resourceID + `","resourceType":"project"}`}
	cr := assignment("", scanner, v1alpha1.Role{Name: "ast-viewer"})
	if _, err := connectTo(t, api).Create(t.Context(), cr); err != nil {
		t.Fatal(err)
	}
	if got := meta.GetExternalName(cr); got != externalName {
		t.Errorf("external-name = %q, want %q", got, externalName)
	}
	assertReceived(t, api, post)
}

func TestCreateRejectedIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusBadRequest, response: `{"message":"invalid role"}`}
	cr := assignment("", scanner)
	_, err := connectTo(t, api).Create(t.Context(), cr)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %v, want an error reporting HTTP 400", err)
	}
	if got := meta.GetExternalName(cr); got != "" {
		t.Errorf("external-name = %q, want it unset", got)
	}
}

func TestUpdateDeletesThenRecreates(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated, response: `{}`}
	if _, err := connectTo(t, api).Update(t.Context(), assignment(externalName, scanner, v1alpha1.Role{Name: "ast-viewer"})); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, del, post)
}

func TestUpdateRejectedDeleteDoesNotRecreate(t *testing.T) {
	api := &fakeAPI{status: http.StatusForbidden, response: `{"message":"forbidden"}`}
	_, err := connectTo(t, api).Update(t.Context(), assignment(externalName, scanner))
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("error = %v, want an error reporting HTTP 403", err)
	}
	assertReceived(t, api, del)
}

func TestDeleteSendsQueryParameterDelete(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	if _, err := connectTo(t, api).Delete(t.Context(), assignment(externalName, scanner)); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, del)
}

func TestDeleteNotFoundSucceeds(t *testing.T) {
	api := &fakeAPI{status: http.StatusNotFound, response: `{"message":"not found"}`}
	if _, err := connectTo(t, api).Delete(t.Context(), assignment(externalName, scanner)); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, del)
}

func TestDeleteRejectedIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusForbidden, response: `{"message":"forbidden"}`}
	_, err := connectTo(t, api).Delete(t.Context(), assignment(externalName, scanner))
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("error = %v, want an error reporting HTTP 403", err)
	}
}

var errTransport = errors.New("connection refused")

type failingTransport struct {
	method string
}

func (f failingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method == f.method {
		return nil, errTransport
	}
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Request: r}, nil
}

func failing(method string) managed.TypedExternalClient[*v1alpha1.AccessAssignment] {
	return newExternal(&clients.Connection{HTTPClient: &http.Client{Transport: failingTransport{method: method}}, BaseURL: "http://checkmarx.invalid"})
}

func TestTransportFailuresAreErrors(t *testing.T) {
	cases := map[string]func(managed.TypedExternalClient[*v1alpha1.AccessAssignment]) error{
		http.MethodGet: func(c managed.TypedExternalClient[*v1alpha1.AccessAssignment]) error {
			_, err := c.Observe(t.Context(), assignment(externalName, scanner))
			return err
		},
		http.MethodPost: func(c managed.TypedExternalClient[*v1alpha1.AccessAssignment]) error {
			_, err := c.Create(t.Context(), assignment("", scanner))
			return err
		},
		http.MethodDelete: func(c managed.TypedExternalClient[*v1alpha1.AccessAssignment]) error {
			_, err := c.Delete(t.Context(), assignment(externalName, scanner))
			return err
		},
	}
	for method, call := range cases {
		t.Run(method, func(t *testing.T) {
			if err := call(failing(method)); !errors.Is(err, errTransport) {
				t.Errorf("error = %v, want %v", err, errTransport)
			}
		})
	}
}

func TestUpdateSurfacesCreateFailureAfterDelete(t *testing.T) {
	if _, err := failing(http.MethodPost).Update(t.Context(), assignment(externalName, scanner)); !errors.Is(err, errTransport) {
		t.Errorf("error = %v, want %v", err, errTransport)
	}
}

func TestObserveDuplicateRoleNamesCollapse(t *testing.T) {
	cases := map[string]struct {
		remote string
		spec   []v1alpha1.Role
	}{
		"RemoteDuplicates": {remote: `[{"name":"ast-scanner"},{"name":"ast-scanner"},{"name":"ast-viewer"}]`, spec: []v1alpha1.Role{scanner, viewer}},
		"SpecDuplicates":   {remote: `[{"name":"ast-scanner"},{"name":"ast-viewer"}]`, spec: []v1alpha1.Role{scanner, scanner, viewer}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{status: http.StatusOK, response: `[{"entityID":"` + entityID + `","entityType":"group","entityRoles":` + tc.remote + `,"resourceID":"` + resourceID + `","resourceType":"project"}]`}
			got, err := connectTo(t, api).Observe(t.Context(), assignment(externalName, tc.spec...))
			if err != nil {
				t.Fatal(err)
			}
			if want := (managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}); !reflect.DeepEqual(got, want) {
				t.Errorf("observation = %+v, want %+v", got, want)
			}
		})
	}
}

func TestObserveSuccessWithoutListBodyIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	_, err := connectTo(t, api).Observe(t.Context(), assignment(externalName, scanner))
	if err == nil || !strings.Contains(err.Error(), "204") {
		t.Errorf("error = %v, want an error reporting HTTP 204", err)
	}
	assertReceived(t, api, listGet)
}

func TestObserveEntryWithoutEntityFieldsDoesNotMatch(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `[
		{"entityType":"group","entityRoles":[{"name":"ast-scanner"}],"resourceID":"` + resourceID + `","resourceType":"project"},
		{"entityID":"` + entityID + `","entityRoles":[{"name":"ast-scanner"}],"resourceID":"` + resourceID + `","resourceType":"project"}
	]`}
	got, err := connectTo(t, api).Observe(t.Context(), assignment(externalName, scanner))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
}
