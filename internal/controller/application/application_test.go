// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/Cliftonz/provider-checkmarx/apis/application/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
)

const (
	appID   = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
	appPath = "/api/applications/" + appID
	remote  = `{"id":"3fa85f64-5717-4562-b3fc-2c963f66afa6","name":"payments","description":"Payments","criticality":3,"type":"standard",` +
		`"rules":[{"id":"9b2c1a40-1111-4c3e-8a7d-0123456789ab","type":"project.name.in","value":"pay-api;pay-web"}],"tags":{"team":"pay"},` +
		`"projectIds":["7c9e6679-7425-40de-944b-e07fc1f90ae7"],"createdAt":"2026-01-02T03:04:05Z","updatedAt":"2026-02-03T04:05:06Z"}`
)

type request struct {
	Method string
	Path   string
	Body   map[string]any
}

type fakeAPI struct {
	status   int
	response string
	received []request
}

func connectTo(t *testing.T, api *fakeAPI) *external {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		api.received = append(api.received, request{Method: r.Method, Path: r.URL.Path, Body: body})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(api.status)
		_, _ = io.WriteString(w, api.response)
	}))
	t.Cleanup(srv.Close)
	return newExternal(&clients.Connection{HTTPClient: srv.Client(), BaseURL: srv.URL}).(*external)
}

func application(externalName string, fp v1alpha1.ApplicationParameters) *v1alpha1.Application {
	cr := &v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{ForProvider: fp}}
	meta.SetExternalName(cr, externalName)
	return cr
}

func fullSpec() v1alpha1.ApplicationParameters {
	return v1alpha1.ApplicationParameters{
		Name:        "payments",
		Description: ptr("Payments"),
		Criticality: ptr(3),
		Rules:       []v1alpha1.ApplicationRule{{Type: "project.name.in", Value: "pay-api;pay-web"}},
		Tags:        map[string]string{"team": "pay"},
		Type:        ptr("standard"),
	}
}

func fullBody() map[string]any {
	return map[string]any{
		"name":        "payments",
		"description": "Payments",
		"criticality": float64(3),
		"type":        "standard",
		"rules":       []any{map[string]any{"type": "project.name.in", "value": "pay-api;pay-web"}},
		"tags":        map[string]any{"team": "pay"},
	}
}

func ptr[T any](v T) *T {
	return &v
}

func assertReceived(t *testing.T, api *fakeAPI, want ...request) {
	t.Helper()
	if !reflect.DeepEqual(api.received, want) {
		t.Errorf("received requests\n got: %#v\nwant: %#v", api.received, want)
	}
}

func TestObserveWithoutExternalNameDoesNotExist(t *testing.T) {
	api := &fakeAPI{}
	got, err := connectTo(t, api).Observe(t.Context(), application("", fullSpec()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
	assertReceived(t, api)
}

func TestObserveNotFoundDoesNotExist(t *testing.T) {
	api := &fakeAPI{status: http.StatusNotFound, response: `{"message":"not found"}`}
	got, err := connectTo(t, api).Observe(t.Context(), application(appID, fullSpec()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
	assertReceived(t, api, request{Method: http.MethodGet, Path: appPath})
}

func TestObserveExternalNameNotUUIDIsAnError(t *testing.T) {
	api := &fakeAPI{}
	_, err := connectTo(t, api).Observe(t.Context(), application("payments", fullSpec()))
	if err == nil || !strings.Contains(err.Error(), `"payments"`) {
		t.Errorf("error = %v, want an error naming the external name", err)
	}
	assertReceived(t, api)
}

func TestObserveNon2xxReportsStatusAndBody(t *testing.T) {
	cases := map[string]struct {
		status int
		code   string
	}{
		"ServerError":     {status: http.StatusInternalServerError, code: "500"},
		"TooManyRequests": {status: http.StatusTooManyRequests, code: "429"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{status: tc.status, response: `{"message":"boom"}`}
			_, err := connectTo(t, api).Observe(t.Context(), application(appID, fullSpec()))
			if err == nil || !strings.Contains(err.Error(), tc.code) || !strings.Contains(err.Error(), "boom") {
				t.Errorf("error = %v, want an error reporting HTTP %s and the body", err, tc.code)
			}
			assertReceived(t, api, request{Method: http.MethodGet, Path: appPath})
		})
	}
}

func TestObserveExistingApplicationIsAvailableAndUpToDate(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: remote}
	cr := application(appID, fullSpec())
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	want := managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
	wantAt := v1alpha1.ApplicationObservation{
		ID:         appID,
		Rules:      []v1alpha1.ObservedApplicationRule{{ID: "9b2c1a40-1111-4c3e-8a7d-0123456789ab", Type: "project.name.in", Value: "pay-api;pay-web"}},
		ProjectIDs: []string{"7c9e6679-7425-40de-944b-e07fc1f90ae7"},
		CreatedAt:  &metav1.Time{Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
		UpdatedAt:  &metav1.Time{Time: time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)},
	}
	if !reflect.DeepEqual(cr.Status.AtProvider, wantAt) {
		t.Errorf("atProvider = %+v, want %+v", cr.Status.AtProvider, wantAt)
	}
	if c := cr.GetCondition(xpv2.TypeReady); c.Reason != xpv2.ReasonAvailable {
		t.Errorf("ready condition = %+v, want Available", c)
	}
	assertReceived(t, api, request{Method: http.MethodGet, Path: appPath})
}

func TestObserveLateInitializesOmittedFields(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: remote}
	cr := application(appID, v1alpha1.ApplicationParameters{Name: "payments"})
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	want := managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(cr.Spec.ForProvider, fullSpec()) {
		t.Errorf("forProvider = %+v, want %+v", cr.Spec.ForProvider, fullSpec())
	}
}

func TestObserveRemoteWithoutOptionalFieldsDoesNotLateInitialize(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `{"id":"3fa85f64-5717-4562-b3fc-2c963f66afa6","name":"payments","rules":[],"tags":{}}`}
	cr := application(appID, v1alpha1.ApplicationParameters{Name: "payments"})
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	want := managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(cr.Spec.ForProvider, v1alpha1.ApplicationParameters{Name: "payments"}) {
		t.Errorf("forProvider = %+v, want only name", cr.Spec.ForProvider)
	}
}

func TestObserveReportsDriftOnEveryField(t *testing.T) {
	cases := map[string]func(*v1alpha1.ApplicationParameters){
		"Name":        func(p *v1alpha1.ApplicationParameters) { p.Name = "billing" },
		"Description": func(p *v1alpha1.ApplicationParameters) { p.Description = ptr("Billing") },
		"Criticality": func(p *v1alpha1.ApplicationParameters) { p.Criticality = ptr(1) },
		"Type":        func(p *v1alpha1.ApplicationParameters) { p.Type = ptr("other") },
		"RuleValue":   func(p *v1alpha1.ApplicationParameters) { p.Rules[0].Value = "pay-api" },
		"RuleAdded": func(p *v1alpha1.ApplicationParameters) {
			p.Rules = append(p.Rules, v1alpha1.ApplicationRule{Type: "project.tag.key.exists", Value: "pay"})
		},
		"TagValue": func(p *v1alpha1.ApplicationParameters) { p.Tags["team"] = "billing" },
		"TagAdded": func(p *v1alpha1.ApplicationParameters) { p.Tags["env"] = "prod" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			spec := fullSpec()
			change(&spec)
			api := &fakeAPI{status: http.StatusOK, response: remote}
			got, err := connectTo(t, api).Observe(t.Context(), application(appID, spec))
			if err != nil {
				t.Fatal(err)
			}
			want := managed.ExternalObservation{ResourceExists: true}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("observation = %+v, want %+v", got, want)
			}
		})
	}
}

func TestCreatePostsApplicationAndSetsExternalNameFromID(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated, response: remote}
	cr := application("", fullSpec())
	if _, err := connectTo(t, api).Create(t.Context(), cr); err != nil {
		t.Fatal(err)
	}
	if got := meta.GetExternalName(cr); got != appID {
		t.Errorf("external-name = %q, want %q", got, appID)
	}
	assertReceived(t, api, request{Method: http.MethodPost, Path: "/api/applications", Body: fullBody()})
}

func TestCreateSendsOnlyNameWhenOptionalFieldsOmitted(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated, response: `{"id":"3fa85f64-5717-4562-b3fc-2c963f66afa6","name":"payments"}`}
	if _, err := connectTo(t, api).Create(t.Context(), application("", v1alpha1.ApplicationParameters{Name: "payments"})); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodPost, Path: "/api/applications", Body: map[string]any{"name": "payments"}})
}

func TestCreateDuplicateNameSurfacesAPIError(t *testing.T) {
	api := &fakeAPI{status: http.StatusBadRequest, response: `{"message":"application name already exists"}`}
	cr := application("", fullSpec())
	_, err := connectTo(t, api).Create(t.Context(), cr)
	if err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "application name already exists") {
		t.Errorf("error = %v, want an error reporting HTTP 400 and the body", err)
	}
	if got := meta.GetExternalName(cr); got != "" {
		t.Errorf("external-name = %q, want empty", got)
	}
	assertReceived(t, api, request{Method: http.MethodPost, Path: "/api/applications", Body: fullBody()})
}

func TestUpdatePutsApplication(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	if _, err := connectTo(t, api).Update(t.Context(), application(appID, fullSpec())); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodPut, Path: appPath, Body: fullBody()})
}

func TestUpdateRejectedReportsStatusAndBody(t *testing.T) {
	api := &fakeAPI{status: http.StatusBadRequest, response: `{"message":"invalid rule"}`}
	_, err := connectTo(t, api).Update(t.Context(), application(appID, fullSpec()))
	if err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "invalid rule") {
		t.Errorf("error = %v, want an error reporting HTTP 400 and the body", err)
	}
}

func TestDeleteSendsDelete(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	if _, err := connectTo(t, api).Delete(t.Context(), application(appID, fullSpec())); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodDelete, Path: appPath})
}

func TestDeleteNotFoundSucceeds(t *testing.T) {
	api := &fakeAPI{status: http.StatusNotFound, response: `{"message":"not found"}`}
	if _, err := connectTo(t, api).Delete(t.Context(), application(appID, fullSpec())); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodDelete, Path: appPath})
}

func TestDeleteRefusedSurfacesAPIError(t *testing.T) {
	api := &fakeAPI{status: http.StatusConflict, response: `{"message":"application has projects"}`}
	_, err := connectTo(t, api).Delete(t.Context(), application(appID, fullSpec()))
	if err == nil || !strings.Contains(err.Error(), "409") || !strings.Contains(err.Error(), "application has projects") {
		t.Errorf("error = %v, want an error reporting HTTP 409 and the body", err)
	}
	assertReceived(t, api, request{Method: http.MethodDelete, Path: appPath})
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func unreachable() *external {
	return newExternal(&clients.Connection{HTTPClient: &http.Client{Transport: failingTransport{}}, BaseURL: "http://checkmarx.invalid"}).(*external)
}

func TestTransportFailureIsAnError(t *testing.T) {
	cases := map[string]func(*external, *v1alpha1.Application) error{
		"Observe": func(e *external, cr *v1alpha1.Application) error { _, err := e.Observe(t.Context(), cr); return err },
		"Create":  func(e *external, cr *v1alpha1.Application) error { _, err := e.Create(t.Context(), cr); return err },
		"Update":  func(e *external, cr *v1alpha1.Application) error { _, err := e.Update(t.Context(), cr); return err },
		"Delete":  func(e *external, cr *v1alpha1.Application) error { _, err := e.Delete(t.Context(), cr); return err },
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			err := call(unreachable(), application(appID, fullSpec()))
			if err == nil || !strings.Contains(err.Error(), "connection refused") {
				t.Errorf("error = %v, want the transport error", err)
			}
		})
	}
}

func TestSuccessWithoutExpectedBodyIsAnError(t *testing.T) {
	observe := func(e *external, cr *v1alpha1.Application) error { _, err := e.Observe(t.Context(), cr); return err }
	create := func(e *external, cr *v1alpha1.Application) error { _, err := e.Create(t.Context(), cr); return err }
	cases := map[string]struct {
		status       int
		response     string
		externalName string
		call         func(*external, *v1alpha1.Application) error
		want         string
	}{
		"ObserveNoContent": {status: http.StatusNoContent, externalName: appID, call: observe, want: "204"},
		"CreateOK":         {status: http.StatusOK, response: remote, call: create, want: "200"},
		"CreateWithoutID":  {status: http.StatusCreated, response: `{"name":"payments"}`, call: create, want: "without an id"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{status: tc.status, response: tc.response}
			cr := application(tc.externalName, fullSpec())
			err := tc.call(connectTo(t, api), cr)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want an error containing %q", err, tc.want)
			}
			if got := meta.GetExternalName(cr); got != tc.externalName {
				t.Errorf("external-name = %q, want %q", got, tc.externalName)
			}
		})
	}
}

func TestObserveRuleWithoutIDLeavesIDEmpty(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `{"id":"3fa85f64-5717-4562-b3fc-2c963f66afa6","name":"payments","rules":[{"type":"project.name.in","value":"pay-api"}]}`}
	cr := application(appID, v1alpha1.ApplicationParameters{Name: "payments"})
	if _, err := connectTo(t, api).Observe(t.Context(), cr); err != nil {
		t.Fatal(err)
	}
	want := []v1alpha1.ObservedApplicationRule{{Type: "project.name.in", Value: "pay-api"}}
	if !reflect.DeepEqual(cr.Status.AtProvider.Rules, want) {
		t.Errorf("atProvider.rules = %+v, want %+v", cr.Status.AtProvider.Rules, want)
	}
}

func TestObserveResponseWithoutIDTakesIDFromExternalName(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `{"name":"payments"}`}
	cr := application(appID, v1alpha1.ApplicationParameters{Name: "payments"})
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	want := managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
	if cr.Status.AtProvider.ID != appID {
		t.Errorf("atProvider.id = %q, want %q", cr.Status.AtProvider.ID, appID)
	}
}

func TestEmptyCollectionsAreOmitted(t *testing.T) {
	empty := v1alpha1.ApplicationParameters{Name: "payments", Rules: []v1alpha1.ApplicationRule{}, Tags: map[string]string{}}
	t.Run("Create", func(t *testing.T) {
		api := &fakeAPI{status: http.StatusCreated, response: `{"id":"3fa85f64-5717-4562-b3fc-2c963f66afa6","name":"payments"}`}
		if _, err := connectTo(t, api).Create(t.Context(), application("", empty)); err != nil {
			t.Fatal(err)
		}
		assertReceived(t, api, request{Method: http.MethodPost, Path: "/api/applications", Body: map[string]any{"name": "payments"}})
	})
	t.Run("Update", func(t *testing.T) {
		api := &fakeAPI{status: http.StatusNoContent}
		if _, err := connectTo(t, api).Update(t.Context(), application(appID, empty)); err != nil {
			t.Fatal(err)
		}
		assertReceived(t, api, request{Method: http.MethodPut, Path: appPath, Body: map[string]any{"name": "payments"}})
	})
	t.Run("ObserveAgainstRemoteWithout", func(t *testing.T) {
		api := &fakeAPI{status: http.StatusOK, response: `{"name":"payments"}`}
		got, err := connectTo(t, api).Observe(t.Context(), application(appID, empty))
		if err != nil {
			t.Fatal(err)
		}
		want := managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("observation = %+v, want %+v", got, want)
		}
	})
}
