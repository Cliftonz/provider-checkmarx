// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/Cliftonz/provider-checkmarx/apis/group/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
)

const (
	groupID    = "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d"
	groupName  = "devs"
	groupsPath = "/auth/admin/realms/acme/groups"
	groupPath  = groupsPath + "/" + groupID
)

type request struct {
	Method string
	Path   string
	Body   map[string]any
}

type fakeAPI struct {
	status   int
	response string
	location string
	received []request
}

func connectTo(t *testing.T, api *fakeAPI) *external {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		api.received = append(api.received, request{Method: r.Method, Path: r.URL.Path, Body: body})
		if api.location != "" {
			w.Header().Set("Location", api.location)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(api.status)
		_, _ = io.WriteString(w, api.response)
	}))
	t.Cleanup(srv.Close)
	return newExternal(&clients.Connection{HTTPClient: srv.Client(), IAMURL: srv.URL, Tenant: "acme"}).(*external)
}

func groupWithExternalName(externalName string) *v1alpha1.Group {
	cr := &v1alpha1.Group{Spec: v1alpha1.GroupSpec{ForProvider: v1alpha1.GroupParameters{Name: groupName}}}
	meta.SetExternalName(cr, externalName)
	return cr
}

func assertReceived(t *testing.T, api *fakeAPI, want ...request) {
	t.Helper()
	if !reflect.DeepEqual(api.received, want) {
		t.Errorf("received requests\n got: %#v\nwant: %#v", api.received, want)
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v, want an error containing %q", err, want)
	}
}

func TestObserveWithoutExternalNameDoesNotExist(t *testing.T) {
	api := &fakeAPI{}
	got, err := connectTo(t, api).Observe(t.Context(), groupWithExternalName(""))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
	assertReceived(t, api)
}

func TestObserveExistingGroup(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `{"id":"` + groupID + `","name":"devs"}`}
	cr := groupWithExternalName(groupID)
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	if want := (managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
	if want := (v1alpha1.GroupObservation{ID: groupID}); !reflect.DeepEqual(cr.Status.AtProvider, want) {
		t.Errorf("atProvider = %+v, want %+v", cr.Status.AtProvider, want)
	}
	if c := cr.GetCondition(xpv2.TypeReady); c.Reason != xpv2.ReasonAvailable {
		t.Errorf("ready condition = %+v, want Available", c)
	}
	assertReceived(t, api, request{Method: http.MethodGet, Path: groupPath})
}

func TestObserveRenamedGroupIsNotUpToDate(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `{"id":"` + groupID + `","name":"ops"}`}
	got, err := connectTo(t, api).Observe(t.Context(), groupWithExternalName(groupID))
	if err != nil {
		t.Fatal(err)
	}
	if want := (managed.ExternalObservation{ResourceExists: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
}

func TestObserveNotFoundDoesNotExist(t *testing.T) {
	api := &fakeAPI{status: http.StatusNotFound, response: `{"error":"Could not find group by id"}`}
	got, err := connectTo(t, api).Observe(t.Context(), groupWithExternalName(groupID))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
	assertReceived(t, api, request{Method: http.MethodGet, Path: groupPath})
}

func TestObserveServerErrorIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusInternalServerError, response: `{"error":"boom"}`}
	_, err := connectTo(t, api).Observe(t.Context(), groupWithExternalName(groupID))
	assertErrorContains(t, err, `500 Internal Server Error: {"error":"boom"}`)
}

func TestObserveNonUUIDExternalNameIsAnError(t *testing.T) {
	api := &fakeAPI{}
	_, err := connectTo(t, api).Observe(t.Context(), groupWithExternalName("devs"))
	assertErrorContains(t, err, `"devs"`)
	assertReceived(t, api)
}

func TestCreateSetsExternalNameFromLocationHeader(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated, location: "https://iam.example.com" + groupPath}
	cr := groupWithExternalName("")
	if _, err := connectTo(t, api).Create(t.Context(), cr); err != nil {
		t.Fatal(err)
	}
	if got := meta.GetExternalName(cr); got != groupID {
		t.Errorf("external name = %q, want %q", got, groupID)
	}
	assertReceived(t, api, request{Method: http.MethodPost, Path: groupsPath, Body: map[string]any{"name": groupName}})
}

func TestCreateWithoutLocationHeaderIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated}
	cr := groupWithExternalName("")
	_, err := connectTo(t, api).Create(t.Context(), cr)
	assertErrorContains(t, err, "Location header")
	if got := meta.GetExternalName(cr); got != "" {
		t.Errorf("external name = %q, want none", got)
	}
}

func TestCreateWithNonUUIDLocationHeaderIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated, location: "https://iam.example.com" + groupsPath + "/devs"}
	cr := groupWithExternalName("")
	_, err := connectTo(t, api).Create(t.Context(), cr)
	assertErrorContains(t, err, "Location header")
	if got := meta.GetExternalName(cr); got != "" {
		t.Errorf("external name = %q, want none", got)
	}
}

func TestCreateConflictIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusConflict, response: `{"errorMessage":"Top level group named 'devs' already exists."}`}
	_, err := connectTo(t, api).Create(t.Context(), groupWithExternalName(""))
	assertErrorContains(t, err, "409")
}

func TestUpdatePutsName(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	if _, err := connectTo(t, api).Update(t.Context(), groupWithExternalName(groupID)); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodPut, Path: groupPath, Body: map[string]any{"name": groupName}})
}

func TestUpdateServerErrorIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusInternalServerError, response: `{"error":"boom"}`}
	_, err := connectTo(t, api).Update(t.Context(), groupWithExternalName(groupID))
	assertErrorContains(t, err, "500")
}

func TestDeleteDeletesGroup(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	if _, err := connectTo(t, api).Delete(t.Context(), groupWithExternalName(groupID)); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodDelete, Path: groupPath})
}

func TestDeleteNotFoundSucceeds(t *testing.T) {
	api := &fakeAPI{status: http.StatusNotFound, response: `{"error":"Could not find group by id"}`}
	if _, err := connectTo(t, api).Delete(t.Context(), groupWithExternalName(groupID)); err != nil {
		t.Errorf("error = %v, want none", err)
	}
}

func TestDeleteServerErrorIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusInternalServerError, response: `{"error":"boom"}`}
	_, err := connectTo(t, api).Delete(t.Context(), groupWithExternalName(groupID))
	assertErrorContains(t, err, "500")
}

func unreachable(t *testing.T) *external {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return newExternal(&clients.Connection{HTTPClient: srv.Client(), IAMURL: srv.URL, Tenant: "acme"}).(*external)
}

func TestTransportFailureIsAnError(t *testing.T) {
	cases := map[string]func(*external) error{
		"Observe": func(e *external) error { _, err := e.Observe(t.Context(), groupWithExternalName(groupID)); return err },
		"Create":  func(e *external) error { _, err := e.Create(t.Context(), groupWithExternalName("")); return err },
		"Update":  func(e *external) error { _, err := e.Update(t.Context(), groupWithExternalName(groupID)); return err },
		"Delete":  func(e *external) error { _, err := e.Delete(t.Context(), groupWithExternalName(groupID)); return err },
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			if err := call(unreachable(t)); err == nil {
				t.Error("error = nil, want the transport error")
			}
		})
	}
}

func TestObserveSuccessWithoutGroupBodyIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	_, err := connectTo(t, api).Observe(t.Context(), groupWithExternalName(groupID))
	assertErrorContains(t, err, "204")
}

func TestCreateReadsGroupIDFromLocationURLPath(t *testing.T) {
	cases := map[string]string{
		"TrailingSlash": "https://iam.example.com" + groupPath + "/",
		"QueryString":   "https://iam.example.com" + groupPath + "?x=1",
	}
	for name, location := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{status: http.StatusCreated, location: location}
			cr := groupWithExternalName("")
			if _, err := connectTo(t, api).Create(t.Context(), cr); err != nil {
				t.Fatal(err)
			}
			if got := meta.GetExternalName(cr); got != groupID {
				t.Errorf("external name = %q, want %q", got, groupID)
			}
		})
	}
}

func TestCreateWithMalformedLocationURLIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated, location: "https://iam.example.com" + groupsPath + "/%zz"}
	cr := groupWithExternalName("")
	_, err := connectTo(t, api).Create(t.Context(), cr)
	assertErrorContains(t, err, "Location header")
	if got := meta.GetExternalName(cr); got != "" {
		t.Errorf("external name = %q, want none", got)
	}
}
