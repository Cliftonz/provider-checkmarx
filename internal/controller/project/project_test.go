// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
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

	"github.com/Cliftonz/provider-checkmarx/apis/project/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
)

const (
	projectID     = "0b6f3a52-6a0e-4c1e-9d5e-2f7c1a9e8b10"
	groupID       = "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d"
	applicationID = "5d4c3b2a-1f0e-4d9c-8b7a-6f5e4d3c2b1a"
	projectName   = "app"
	projectPath   = "/api/projects/" + projectID
	remoteProject = `{"id":"` + projectID + `","name":"app","repoUrl":"https://git.example/app.git","mainBranch":"main","origin":"crossplane","scmRepoId":"42","tags":{"team":"a"},"groups":["` + groupID + `"],"criticality":3,"applicationIds":["` + applicationID + `"],"createdAt":"2026-11-01T10:00:00Z","updatedAt":"2026-11-02T10:00:00Z"}`
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

func connectTo(t *testing.T, api *fakeAPI) managed.TypedExternalClient[*v1alpha1.Project] {
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
	return newExternal(&clients.Connection{HTTPClient: srv.Client(), BaseURL: srv.URL})
}

func project(externalName string, fp v1alpha1.ProjectParameters) *v1alpha1.Project {
	cr := &v1alpha1.Project{Spec: v1alpha1.ProjectSpec{ForProvider: fp}}
	meta.SetExternalName(cr, externalName)
	return cr
}

func fullSpec() v1alpha1.ProjectParameters {
	return v1alpha1.ProjectParameters{
		Name:        projectName,
		RepoURL:     new("https://git.example/app.git"),
		MainBranch:  new("main"),
		Origin:      new("crossplane"),
		ScmRepoID:   new("42"),
		Tags:        map[string]string{"team": "a"},
		Groups:      []string{groupID},
		Criticality: new(3),
	}
}

func assertReceived(t *testing.T, api *fakeAPI, want ...request) {
	t.Helper()
	if !reflect.DeepEqual(api.received, want) {
		t.Errorf("received requests\n got: %#v\nwant: %#v", api.received, want)
	}
}

func TestObserveWithoutExternalNameDoesNotExist(t *testing.T) {
	api := &fakeAPI{}
	got, err := connectTo(t, api).Observe(t.Context(), project("", fullSpec()))
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
	got, err := connectTo(t, api).Observe(t.Context(), project(projectID, fullSpec()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
	assertReceived(t, api, request{Method: http.MethodGet, Path: projectPath})
}

func TestObserveServerErrorIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusInternalServerError, response: `{"message":"boom"}`}
	_, err := connectTo(t, api).Observe(t.Context(), project(projectID, fullSpec()))
	if err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want an error reporting HTTP 500 and the body", err)
	}
}

func TestObserveExternalNameThatIsNotAUUIDIsAnError(t *testing.T) {
	api := &fakeAPI{}
	_, err := connectTo(t, api).Observe(t.Context(), project(projectName, fullSpec()))
	if err == nil {
		t.Error("error = nil, want an error for an external name that is not a UUID")
	}
	assertReceived(t, api)
}

func TestObserveExistingProjectSetsAtProviderAndIsUpToDate(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: remoteProject}
	spec := fullSpec()
	spec.PrivatePackage = new(true)
	cr := project(projectID, spec)
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	if want := (managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
	wantAt := v1alpha1.ProjectObservation{
		ID:             projectID,
		CreatedAt:      &metav1.Time{Time: time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)},
		UpdatedAt:      &metav1.Time{Time: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)},
		ApplicationIDs: []string{applicationID},
	}
	if !reflect.DeepEqual(cr.Status.AtProvider, wantAt) {
		t.Errorf("atProvider = %+v, want %+v", cr.Status.AtProvider, wantAt)
	}
	if c := cr.GetCondition(xpv2.TypeReady); c.Reason != xpv2.ReasonAvailable {
		t.Errorf("ready condition = %+v, want Available", c)
	}
	assertReceived(t, api, request{Method: http.MethodGet, Path: projectPath})
}

func TestObserveUpToDateComparesEveryFieldButPrivatePackage(t *testing.T) {
	cases := map[string]struct {
		change func(*v1alpha1.ProjectParameters)
		want   bool
	}{
		"Name":           {change: func(p *v1alpha1.ProjectParameters) { p.Name = "other" }},
		"RepoURL":        {change: func(p *v1alpha1.ProjectParameters) { p.RepoURL = new("https://git.example/other.git") }},
		"MainBranch":     {change: func(p *v1alpha1.ProjectParameters) { p.MainBranch = new("dev") }},
		"Origin":         {change: func(p *v1alpha1.ProjectParameters) { p.Origin = new("other") }},
		"ScmRepoID":      {change: func(p *v1alpha1.ProjectParameters) { p.ScmRepoID = new("43") }},
		"Tags":           {change: func(p *v1alpha1.ProjectParameters) { p.Tags = map[string]string{"team": "b"} }},
		"Groups":         {change: func(p *v1alpha1.ProjectParameters) { p.Groups = []string{applicationID} }},
		"Criticality":    {change: func(p *v1alpha1.ProjectParameters) { p.Criticality = new(1) }},
		"PrivatePackage": {change: func(p *v1alpha1.ProjectParameters) { p.PrivatePackage = new(false) }, want: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{status: http.StatusOK, response: remoteProject}
			spec := fullSpec()
			tc.change(&spec)
			got, err := connectTo(t, api).Observe(t.Context(), project(projectID, spec))
			if err != nil {
				t.Fatal(err)
			}
			if got.ResourceUpToDate != tc.want {
				t.Errorf("ResourceUpToDate = %v, want %v", got.ResourceUpToDate, tc.want)
			}
		})
	}
}

func TestObserveLateInitializesOmittedFieldsExceptPrivatePackage(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: remoteProject}
	cr := project(projectID, v1alpha1.ProjectParameters{Name: projectName})
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	if want := (managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(cr.Spec.ForProvider, fullSpec()) {
		t.Errorf("forProvider = %+v, want %+v", cr.Spec.ForProvider, fullSpec())
	}
}

func TestObserveRemoteWithoutOptionalFieldsLeavesThemUnset(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `{"id":"` + projectID + `","name":"app"}`}
	cr := project(projectID, v1alpha1.ProjectParameters{Name: projectName})
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	if want := (managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
	if want := (v1alpha1.ProjectParameters{Name: projectName}); !reflect.DeepEqual(cr.Spec.ForProvider, want) {
		t.Errorf("forProvider = %+v, want %+v", cr.Spec.ForProvider, want)
	}
	if want := (v1alpha1.ProjectObservation{ID: projectID}); !reflect.DeepEqual(cr.Status.AtProvider, want) {
		t.Errorf("atProvider = %+v, want %+v", cr.Status.AtProvider, want)
	}
}

func wantBody() map[string]any {
	return map[string]any{
		"name":           projectName,
		"repoUrl":        "https://git.example/app.git",
		"mainBranch":     "main",
		"origin":         "crossplane",
		"scmRepoId":      "42",
		"tags":           map[string]any{"team": "a"},
		"groups":         []any{groupID},
		"criticality":    float64(3),
		"privatePackage": true,
	}
}

func TestCreatePostsProjectAndSetsExternalNameFromResponseID(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated, response: remoteProject}
	spec := fullSpec()
	spec.PrivatePackage = new(true)
	cr := project("", spec)
	if _, err := connectTo(t, api).Create(t.Context(), cr); err != nil {
		t.Fatal(err)
	}
	if got := meta.GetExternalName(cr); got != projectID {
		t.Errorf("external-name = %q, want %q", got, projectID)
	}
	assertReceived(t, api, request{Method: http.MethodPost, Path: "/api/projects", Body: wantBody()})
}

func TestCreateDuplicateNameSurfacesTheAPIError(t *testing.T) {
	api := &fakeAPI{status: http.StatusBadRequest, response: `{"message":"project name app already exists"}`}
	_, err := connectTo(t, api).Create(t.Context(), project("", fullSpec()))
	if err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %v, want the API's HTTP 400 duplicate-name error", err)
	}
}

func TestUpdatePutsProject(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	spec := fullSpec()
	spec.PrivatePackage = new(true)
	if _, err := connectTo(t, api).Update(t.Context(), project(projectID, spec)); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodPut, Path: projectPath, Body: wantBody()})
}

func TestUpdateRejectedIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusBadRequest, response: `{"message":"invalid"}`}
	_, err := connectTo(t, api).Update(t.Context(), project(projectID, fullSpec()))
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %v, want an error reporting HTTP 400", err)
	}
}

func TestDeleteSendsDelete(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	if _, err := connectTo(t, api).Delete(t.Context(), project(projectID, fullSpec())); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodDelete, Path: projectPath})
}

func TestDeleteNotFoundSucceeds(t *testing.T) {
	api := &fakeAPI{status: http.StatusNotFound, response: `{"message":"not found"}`}
	if _, err := connectTo(t, api).Delete(t.Context(), project(projectID, fullSpec())); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodDelete, Path: projectPath})
}

func TestDeleteRejectedIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusForbidden, response: `{"message":"forbidden"}`}
	_, err := connectTo(t, api).Delete(t.Context(), project(projectID, fullSpec()))
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("error = %v, want an error reporting HTTP 403", err)
	}
}

func TestTransportFailureIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	e := newExternal(&clients.Connection{HTTPClient: http.DefaultClient, BaseURL: srv.URL})
	calls := map[string]func() error{
		"Observe": func() error { _, err := e.Observe(t.Context(), project(projectID, fullSpec())); return err },
		"Create":  func() error { _, err := e.Create(t.Context(), project("", fullSpec())); return err },
		"Update":  func() error { _, err := e.Update(t.Context(), project(projectID, fullSpec())); return err },
		"Delete":  func() error { _, err := e.Delete(t.Context(), project(projectID, fullSpec())); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("error = nil, want the transport error")
			}
		})
	}
}

func TestObserveComparesGroupsAsASet(t *testing.T) {
	remote := `{"id":"` + projectID + `","name":"app","groups":["` + groupID + `","` + applicationID + `"]}`
	cases := map[string]struct {
		groups []string
		want   bool
	}{
		"ReversedOrder":     {groups: []string{applicationID, groupID}, want: true},
		"DifferentMembers":  {groups: []string{groupID, projectID}},
		"MissingOneMember":  {groups: []string{groupID}},
		"SameMembersInSync": {groups: []string{groupID, applicationID}, want: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{status: http.StatusOK, response: remote}
			got, err := connectTo(t, api).Observe(t.Context(), project(projectID, v1alpha1.ProjectParameters{Name: projectName, Groups: tc.groups}))
			if err != nil {
				t.Fatal(err)
			}
			if got.ResourceUpToDate != tc.want {
				t.Errorf("ResourceUpToDate = %v, want %v", got.ResourceUpToDate, tc.want)
			}
		})
	}
}

func TestObserveSuccessWithoutBodyIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusNoContent}
	_, err := connectTo(t, api).Observe(t.Context(), project(projectID, fullSpec()))
	if err == nil || !strings.Contains(err.Error(), "204") {
		t.Errorf("error = %v, want an error naming HTTP 204", err)
	}
}

func TestCreateSuccessWithoutCreatedBodyIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: remoteProject}
	_, err := connectTo(t, api).Create(t.Context(), project("", fullSpec()))
	if err == nil || !strings.Contains(err.Error(), "200") {
		t.Errorf("error = %v, want an error naming HTTP 200", err)
	}
}

func TestCreateResponseWithoutIDIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated, response: `{"name":"app"}`}
	cr := project("", fullSpec())
	_, err := connectTo(t, api).Create(t.Context(), cr)
	if err == nil || !strings.Contains(err.Error(), "id") {
		t.Errorf("error = %v, want an error naming the missing id", err)
	}
	if got := meta.GetExternalName(cr); got != "" {
		t.Errorf("external-name = %q, want it unset", got)
	}
}

func TestObserveIgnoresGroupReferencesWhenComparing(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: remoteProject}
	spec := fullSpec()
	spec.GroupsRef = []xpv2.NamespacedReference{{Name: "developers"}}
	spec.GroupsSelector = &xpv2.NamespacedSelector{MatchLabels: map[string]string{"team": "a"}}
	got, err := connectTo(t, api).Observe(t.Context(), project(projectID, spec))
	if err != nil {
		t.Fatal(err)
	}
	if !got.ResourceUpToDate {
		t.Error("ResourceUpToDate = false, want true")
	}
}

func TestCreateOmitsUnsetTagsAndGroups(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated, response: remoteProject}
	if _, err := connectTo(t, api).Create(t.Context(), project("", v1alpha1.ProjectParameters{Name: projectName})); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodPost, Path: "/api/projects", Body: map[string]any{"name": projectName}})
}

func TestObserveEmptyRemoteCollectionsMatchOmittedSpec(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `{"id":"` + projectID + `","name":"app","tags":{},"groups":[]}`}
	cr := project(projectID, v1alpha1.ProjectParameters{Name: projectName})
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	if want := (managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
	if cr.Spec.ForProvider.Tags != nil || cr.Spec.ForProvider.Groups != nil {
		t.Errorf("forProvider tags = %#v, groups = %#v, want both nil", cr.Spec.ForProvider.Tags, cr.Spec.ForProvider.Groups)
	}
}

func TestObserveEmptySpecCollectionsMatchAbsentRemote(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `{"id":"` + projectID + `","name":"app"}`}
	cr := project(projectID, v1alpha1.ProjectParameters{Name: projectName, Tags: map[string]string{}, Groups: []string{}})
	got, err := connectTo(t, api).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ResourceUpToDate {
		t.Error("ResourceUpToDate = false, want true")
	}
}

func TestCreateOmitsEmptyTagsAndGroups(t *testing.T) {
	api := &fakeAPI{status: http.StatusCreated, response: remoteProject}
	if _, err := connectTo(t, api).Create(t.Context(), project("", v1alpha1.ProjectParameters{Name: projectName, Tags: map[string]string{}, Groups: []string{}})); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api, request{Method: http.MethodPost, Path: "/api/projects", Body: map[string]any{"name": projectName}})
}

func TestObserveRemoteWithoutNameIsDrift(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: `{"id":"` + projectID + `"}`}
	got, err := connectTo(t, api).Observe(t.Context(), project(projectID, v1alpha1.ProjectParameters{Name: projectName}))
	if err != nil {
		t.Fatal(err)
	}
	if want := (managed.ExternalObservation{ResourceExists: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
}
