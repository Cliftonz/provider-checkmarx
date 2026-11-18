// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package projectconfiguration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/Cliftonz/provider-checkmarx/apis/projectconfiguration/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/configuration"
)

const (
	projectID  = "0b6f3a52-6a0e-4c1e-9d5e-2f7c1a9e8b10"
	configPath = "/api/configuration/project"
	presetKey  = "scan.config.sast.presetName"
	branchKey  = "scan.handler.git.branch"
)

type request struct {
	Method    string
	Path      string
	ProjectID string
	Body      []configuration.ConfigurationEntry
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
		var body []configuration.ConfigurationEntry
		_ = json.Unmarshal(raw, &body)
		api.received = append(api.received, request{Method: r.Method, Path: r.URL.Path, ProjectID: r.URL.Query().Get("project-id"), Body: body})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(api.status)
		_, _ = io.WriteString(w, api.response)
	}))
	t.Cleanup(srv.Close)
	c, err := configuration.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &external{client: c}
}

func projectConfiguration(externalName string, entries ...v1alpha1.ConfigurationEntry) *v1alpha1.ProjectConfiguration {
	cr := &v1alpha1.ProjectConfiguration{Spec: v1alpha1.ProjectConfigurationSpec{ForProvider: v1alpha1.ProjectConfigurationParameters{ProjectID: projectID, Entries: entries}}}
	meta.SetExternalName(cr, externalName)
	return cr
}

func assertReceived(t *testing.T, api *fakeAPI, want ...request) {
	t.Helper()
	if !reflect.DeepEqual(api.received, want) {
		t.Errorf("received requests\n got: %#v\nwant: %#v", api.received, want)
	}
}

var preset = v1alpha1.ConfigurationEntry{Key: presetKey, Value: "ASA Premium"}

func TestObserveWithoutExternalNameDoesNotExist(t *testing.T) {
	api := &fakeAPI{}
	got, err := connectTo(t, api).Observe(t.Context(), projectConfiguration("", preset))
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
	got, err := connectTo(t, api).Observe(t.Context(), projectConfiguration(projectID, preset))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
	assertReceived(t, api, request{Method: http.MethodGet, Path: configPath, ProjectID: projectID})
}

func TestObserveServerErrorIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusInternalServerError, response: `{"message":"boom"}`}
	_, err := connectTo(t, api).Observe(t.Context(), projectConfiguration(projectID, preset))
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %v, want an error reporting HTTP 500", err)
	}
}

func TestObserveMalformedExternalNameIsAnError(t *testing.T) {
	api := &fakeAPI{}
	_, err := connectTo(t, api).Observe(t.Context(), projectConfiguration("not-a-uuid", preset))
	if err == nil {
		t.Error("error = nil, want an error for an external name that is not a UUID")
	}
	assertReceived(t, api)
}

func TestObserveComparesOnlyListedKeys(t *testing.T) {
	cases := map[string]struct {
		remote string
		want   bool
	}{
		"ListedKeysMatch": {
			remote: `[{"key":"scan.config.sast.presetName","value":"ASA Premium"}]`,
			want:   true,
		},
		"ListedValueDiffers": {
			remote: `[{"key":"scan.config.sast.presetName","value":"Default"}]`,
			want:   false,
		},
		"UnlistedServerKeysIgnored": {
			remote: `[{"key":"scan.handler.git.branch","value":"main"},{"key":"scan.config.sast.presetName","value":"ASA Premium"}]`,
			want:   true,
		},
		"ListedKeyAbsent": {
			remote: `[{"key":"scan.handler.git.branch","value":"main"}]`,
			want:   false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{status: http.StatusOK, response: tc.remote}
			cr := projectConfiguration(projectID, preset)
			got, err := connectTo(t, api).Observe(t.Context(), cr)
			if err != nil {
				t.Fatal(err)
			}
			if want := (managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: tc.want}); !reflect.DeepEqual(got, want) {
				t.Errorf("observation = %+v, want %+v", got, want)
			}
			if c := cr.GetCondition(xpv2.TypeReady); c.Reason != xpv2.ReasonAvailable {
				t.Errorf("ready condition = %+v, want Available", c)
			}
			assertReceived(t, api, request{Method: http.MethodGet, Path: configPath, ProjectID: projectID})
		})
	}
}

func TestCreateAdoptsTheProjectSingleton(t *testing.T) {
	api := &fakeAPI{}
	cr := projectConfiguration("", preset)
	if _, err := connectTo(t, api).Create(t.Context(), cr); err != nil {
		t.Fatal(err)
	}
	if got := meta.GetExternalName(cr); got != projectID {
		t.Errorf("external name = %q, want %q", got, projectID)
	}
	assertReceived(t, api)
}

func TestUpdatePatchesListedEntries(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	branch := v1alpha1.ConfigurationEntry{Key: branchKey, Value: "main"}
	if _, err := connectTo(t, api).Update(t.Context(), projectConfiguration(projectID, preset, branch)); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodPatch, Path: configPath, ProjectID: projectID, Body: []configuration.ConfigurationEntry{
		{Key: presetKey, Value: "ASA Premium"},
		{Key: branchKey, Value: "main"},
	}})
}

func TestUpdateServerErrorIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusBadRequest, response: `{"message":"bad key"}`}
	_, err := connectTo(t, api).Update(t.Context(), projectConfiguration(projectID, preset))
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %v, want an error reporting HTTP 400", err)
	}
}

func TestDeleteMakesNoRequest(t *testing.T) {
	api := &fakeAPI{}
	if _, err := connectTo(t, api).Delete(t.Context(), projectConfiguration(projectID, preset)); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api)
}

func unreachable(t *testing.T) *external {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	c, err := configuration.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &external{client: c}
}

func TestObserveTransportErrorIsAnError(t *testing.T) {
	if _, err := unreachable(t).Observe(t.Context(), projectConfiguration(projectID, preset)); err == nil {
		t.Error("error = nil, want the transport error")
	}
}

func TestUpdateTransportErrorIsAnError(t *testing.T) {
	if _, err := unreachable(t).Update(t.Context(), projectConfiguration(projectID, preset)); err == nil {
		t.Error("error = nil, want the transport error")
	}
}

func TestObserveDeletingDoesNotExist(t *testing.T) {
	api := &fakeAPI{}
	cr := projectConfiguration(projectID, preset)
	cr.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
	assertReceived(t, api)
}

func TestObserveSuccessWithoutBodyIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	_, err := connectTo(t, api).Observe(t.Context(), projectConfiguration(projectID, preset))
	if err == nil || !strings.Contains(err.Error(), "204") {
		t.Errorf("error = %v, want an error reporting HTTP 204", err)
	}
}

func TestObserveListedEmptyValueAbsentOnServerIsDrift(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `[{"key":"scan.handler.git.branch","value":"main"}]`}
	got, err := connectTo(t, api).Observe(t.Context(), projectConfiguration(projectID, v1alpha1.ConfigurationEntry{Key: presetKey, Value: ""}))
	if err != nil {
		t.Fatal(err)
	}
	if want := (managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}); !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
}
