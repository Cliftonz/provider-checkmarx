// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package tenantconfiguration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/Cliftonz/provider-checkmarx/apis/tenantconfiguration/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
)

const tenantPath = "/api/configuration/tenant"

type request struct {
	Method string
	Path   string
	Body   []map[string]string
}

type fakeAPI struct {
	status   int
	response string
	received []request
}

func connectTo(t *testing.T, api *fakeAPI) managed.TypedExternalClient[*v1alpha1.TenantConfiguration] {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body []map[string]string
		_ = json.Unmarshal(raw, &body)
		api.received = append(api.received, request{Method: r.Method, Path: r.URL.Path, Body: body})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(api.status)
		_, _ = io.WriteString(w, api.response)
	}))
	t.Cleanup(srv.Close)
	return newExternal(&clients.Connection{HTTPClient: srv.Client(), BaseURL: srv.URL})
}

func tenantConfiguration(externalName string, entries ...v1alpha1.ConfigurationEntry) *v1alpha1.TenantConfiguration {
	cr := &v1alpha1.TenantConfiguration{Spec: v1alpha1.TenantConfigurationSpec{ForProvider: v1alpha1.TenantConfigurationParameters{Entries: entries}}}
	meta.SetExternalName(cr, externalName)
	return cr
}

func assertReceived(t *testing.T, api *fakeAPI, want ...request) {
	t.Helper()
	if !reflect.DeepEqual(api.received, want) {
		t.Errorf("received requests\n got: %#v\nwant: %#v", api.received, want)
	}
}

func observe(t *testing.T, response string, entries ...v1alpha1.ConfigurationEntry) (managed.ExternalObservation, *v1alpha1.TenantConfiguration) {
	t.Helper()
	api := &fakeAPI{status: http.StatusOK, response: response}
	cr := tenantConfiguration("tenant", entries...)
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodGet, Path: tenantPath})
	return got, cr
}

var presetName = v1alpha1.ConfigurationEntry{Key: "scan.config.sast.presetName", Value: "ASA Premium"}

func TestObserveWithoutExternalNameDoesNotExist(t *testing.T) {
	api := &fakeAPI{}
	got, err := connectTo(t, api).Observe(t.Context(), tenantConfiguration("", presetName))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("got %#v, want a nonexistent resource", got)
	}
	assertReceived(t, api)
}

func TestObserveMatchingListedKeysIsUpToDate(t *testing.T) {
	got, cr := observe(t, `[{"key":"scan.config.sast.presetName","value":"ASA Premium"}]`, presetName)
	if !got.ResourceExists || !got.ResourceUpToDate {
		t.Errorf("got %#v, want an existing up-to-date resource", got)
	}
	if !cr.GetCondition(xpv2.TypeReady).Equal(xpv2.Available()) {
		t.Errorf("got Ready condition %#v, want Available", cr.GetCondition(xpv2.TypeReady))
	}
}

func TestObserveDifferingListedValueIsDrift(t *testing.T) {
	got, _ := observe(t, `[{"key":"scan.config.sast.presetName","value":"Checkmarx Default"}]`, presetName)
	if !got.ResourceExists || got.ResourceUpToDate {
		t.Errorf("got %#v, want an existing resource with drift", got)
	}
}

func TestObserveIgnoresUnlistedServerKeys(t *testing.T) {
	got, _ := observe(t, `[{"key":"scan.config.sast.presetName","value":"ASA Premium"},{"key":"scan.config.sca.exploitablePath","value":"true"}]`, presetName)
	if !got.ResourceExists || !got.ResourceUpToDate {
		t.Errorf("got %#v, want an existing up-to-date resource", got)
	}
}

func TestObserveNotFoundDoesNotExist(t *testing.T) {
	api := &fakeAPI{status: http.StatusNotFound}
	got, err := connectTo(t, api).Observe(t.Context(), tenantConfiguration("tenant", presetName))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("got %#v, want a nonexistent resource", got)
	}
}

func TestObserveServerErrorIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusInternalServerError, response: `{"message":"boom"}`}
	if _, err := connectTo(t, api).Observe(t.Context(), tenantConfiguration("tenant", presetName)); err == nil {
		t.Error("want an error for a 500 response")
	}
}

func TestCreateSetsExternalNameWithoutRequest(t *testing.T) {
	api := &fakeAPI{}
	cr := tenantConfiguration("", presetName)
	if _, err := connectTo(t, api).Create(t.Context(), cr); err != nil {
		t.Fatal(err)
	}
	if got := meta.GetExternalName(cr); got != "tenant" {
		t.Errorf("got external name %q, want %q", got, "tenant")
	}
	assertReceived(t, api)
}

func TestUpdatePatchesListedEntries(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	exploitablePath := v1alpha1.ConfigurationEntry{Key: "scan.config.sca.exploitablePath", Value: "true"}
	if _, err := connectTo(t, api).Update(t.Context(), tenantConfiguration("tenant", presetName, exploitablePath)); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodPatch, Path: tenantPath, Body: []map[string]string{
		{"key": "scan.config.sast.presetName", "value": "ASA Premium"},
		{"key": "scan.config.sca.exploitablePath", "value": "true"},
	}})
}

func TestUpdateRejectedIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusBadRequest, response: `{"message":"bad key"}`}
	if _, err := connectTo(t, api).Update(t.Context(), tenantConfiguration("tenant", presetName)); err == nil {
		t.Error("want an error for a 400 response")
	}
}

func TestDeleteSendsNoRequest(t *testing.T) {
	api := &fakeAPI{}
	if _, err := connectTo(t, api).Delete(t.Context(), tenantConfiguration("tenant", presetName)); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api)
}

func unreachable(t *testing.T) managed.TypedExternalClient[*v1alpha1.TenantConfiguration] {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return newExternal(&clients.Connection{HTTPClient: srv.Client(), BaseURL: srv.URL})
}

func TestObserveTransportErrorIsAnError(t *testing.T) {
	if _, err := unreachable(t).Observe(t.Context(), tenantConfiguration("tenant", presetName)); err == nil {
		t.Error("want an error when the server is unreachable")
	}
}

func TestUpdateTransportErrorIsAnError(t *testing.T) {
	if _, err := unreachable(t).Update(t.Context(), tenantConfiguration("tenant", presetName)); err == nil {
		t.Error("want an error when the server is unreachable")
	}
}

func TestObserveAbsentListedKeyIsDrift(t *testing.T) {
	got, _ := observe(t, `[]`, v1alpha1.ConfigurationEntry{Key: "scan.config.sast.filter", Value: ""})
	if !got.ResourceExists || got.ResourceUpToDate {
		t.Errorf("got %#v, want an existing resource with drift", got)
	}
}

func TestObserveDeletingDoesNotExist(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `[]`}
	cr := tenantConfiguration("tenant", presetName)
	cr.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("got %#v, want a nonexistent resource", got)
	}
	assertReceived(t, api)
}

func TestObserveSuccessWithoutBodyIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	_, err := connectTo(t, api).Observe(t.Context(), tenantConfiguration("tenant", presetName))
	if err == nil || !strings.Contains(err.Error(), "204") {
		t.Errorf("error = %v, want an error naming 204", err)
	}
}
