// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package projectgitsettings

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/Cliftonz/provider-checkmarx/apis/projectgitsettings/v1alpha1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients/projects"
)

const (
	projectID  = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
	gitPath    = "/api/projects/" + projectID + "/sourceCode/remoteSettings/git"
	repoURL    = "https://github.com/acme/app.git"
	branch     = "main"
	privateKey = "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----\n"
	remoteGit  = `{"url":"https://github.com/acme/app.git","branch":"main"}`
	secretName = "app-deploy-key"
	secretKey  = "privateKey"
	namespace  = "team-a"
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

func connect(t *testing.T, api *fakeAPI, secrets ...*corev1.Secret) managed.TypedExternalClient[*v1alpha1.ProjectGitSettings] {
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
	c, err := projects.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	b := kfake.NewClientBuilder()
	for _, s := range secrets {
		b = b.WithObjects(s)
	}
	return &external{kube: b.Build(), client: c}
}

func gitSettings(externalName string) *v1alpha1.ProjectGitSettings {
	cr := &v1alpha1.ProjectGitSettings{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
		Spec: v1alpha1.ProjectGitSettingsSpec{ForProvider: v1alpha1.ProjectGitSettingsParameters{
			ProjectID: projectID,
			URL:       repoURL,
			Branch:    branch,
			PrivateKey: xpv2.LocalSecretKeySelector{
				LocalSecretReference: xpv2.LocalSecretReference{Name: secretName},
				Key:                  secretKey,
			},
		}},
	}
	meta.SetExternalName(cr, externalName)
	return cr
}

func applied(externalName string) *v1alpha1.ProjectGitSettings {
	cr := gitSettings(externalName)
	cr.Status.AtProvider.PrivateKeyHash = hash(privateKey)
	return cr
}

func keySecret(key string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
		Data:       map[string][]byte{secretKey: []byte(key)},
	}
}

func hash(key string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
}

func assertReceived(t *testing.T, api *fakeAPI, want ...request) {
	t.Helper()
	if !reflect.DeepEqual(api.received, want) {
		t.Errorf("received requests\n got: %#v\nwant: %#v", api.received, want)
	}
}

var wantPost = request{Method: http.MethodPost, Path: gitPath, Body: map[string]any{"url": repoURL, "branch": branch, "privateKey": privateKey}}

func TestObserveWithoutExternalNameDoesNotExist(t *testing.T) {
	api := &fakeAPI{}
	got, err := connect(t, api, keySecret(privateKey)).Observe(t.Context(), gitSettings(""))
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
	got, err := connect(t, api, keySecret(privateKey)).Observe(t.Context(), applied(projectID))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, managed.ExternalObservation{}) {
		t.Errorf("observation = %+v, want resource not to exist", got)
	}
	assertReceived(t, api, request{Method: http.MethodGet, Path: gitPath})
}

func TestObserveDeletedDoesNotExistWithoutRequestOrSecret(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: remoteGit}
	cr := applied(projectID)
	cr.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
	got, err := connect(t, api).Observe(t.Context(), cr)
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
	_, err := connect(t, api, keySecret(privateKey)).Observe(t.Context(), applied(projectID))
	if err == nil || !strings.Contains(err.Error(), "204") {
		t.Errorf("error = %v, want an error reporting HTTP 204", err)
	}
}

func TestObserveMatchingSettingsAreUpToDateAndAvailable(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: remoteGit}
	cr := applied(projectID)
	got, err := connect(t, api, keySecret(privateKey)).Observe(t.Context(), cr)
	if err != nil {
		t.Fatal(err)
	}
	if want := (managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
	if cr.Status.AtProvider.PrivateKeyHash != hash(privateKey) {
		t.Errorf("atProvider.privateKeyHash = %q, want it unchanged", cr.Status.AtProvider.PrivateKeyHash)
	}
	if c := cr.GetCondition(xpv2.TypeReady); c.Reason != xpv2.ReasonAvailable {
		t.Errorf("ready condition = %+v, want Available", c)
	}
	assertReceived(t, api, request{Method: http.MethodGet, Path: gitPath})
}

func TestObserveDifferentURLOrBranchIsDrift(t *testing.T) {
	for name, remote := range map[string]string{
		"URL":    `{"url":"https://github.com/acme/other.git","branch":"main"}`,
		"Branch": `{"url":"https://github.com/acme/app.git","branch":"develop"}`,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := connect(t, &fakeAPI{status: http.StatusOK, response: remote}, keySecret(privateKey)).Observe(t.Context(), applied(projectID))
			if err != nil {
				t.Fatal(err)
			}
			if want := (managed.ExternalObservation{ResourceExists: true}); !reflect.DeepEqual(got, want) {
				t.Errorf("observation = %+v, want %+v", got, want)
			}
		})
	}
}

func TestObserveChangedSecretValueIsDrift(t *testing.T) {
	got, err := connect(t, &fakeAPI{status: http.StatusOK, response: remoteGit}, keySecret("rotated")).Observe(t.Context(), applied(projectID))
	if err != nil {
		t.Fatal(err)
	}
	if want := (managed.ExternalObservation{ResourceExists: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("observation = %+v, want %+v", got, want)
	}
}

func TestObserveServerErrorIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusInternalServerError, response: `{"message":"boom"}`}
	_, err := connect(t, api, keySecret(privateKey)).Observe(t.Context(), applied(projectID))
	if err == nil || !strings.Contains(err.Error(), projectID) || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want an error naming the project and reporting HTTP 500 with its body", err)
	}
}

func TestObserveSecretReadErrorsAreErrors(t *testing.T) {
	missingKey := keySecret(privateKey)
	missingKey.Data = map[string][]byte{"other": []byte(privateKey)}
	for name, tc := range map[string]struct {
		secrets []*corev1.Secret
		want    func(error) bool
	}{
		"MissingSecret": {want: kerrors.IsNotFound},
		"MissingKey": {secrets: []*corev1.Secret{missingKey}, want: func(err error) bool {
			return err != nil && strings.Contains(err.Error(), `key "privateKey"`)
		}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := connect(t, &fakeAPI{status: http.StatusOK, response: remoteGit}, tc.secrets...).Observe(t.Context(), applied(projectID))
			if !tc.want(err) {
				t.Errorf("error = %v, want the Secret read error", err)
			}
		})
	}
}

func TestCreatePostsKeyFromSecretAndSetsExternalNameAndHash(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK}
	cr := gitSettings("")
	if _, err := connect(t, api, keySecret(privateKey)).Create(t.Context(), cr); err != nil {
		t.Fatal(err)
	}
	if got := meta.GetExternalName(cr); got != projectID {
		t.Errorf("external name = %q, want %q", got, projectID)
	}
	if cr.Status.AtProvider.PrivateKeyHash != hash(privateKey) {
		t.Errorf("atProvider.privateKeyHash = %q, want %q", cr.Status.AtProvider.PrivateKeyHash, hash(privateKey))
	}
	assertReceived(t, api, wantPost)
}

func TestCreateServerErrorIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusBadRequest, response: `{"message":"invalid url"}`}
	cr := gitSettings("")
	_, err := connect(t, api, keySecret(privateKey)).Create(t.Context(), cr)
	if err == nil || !strings.Contains(err.Error(), projectID) || !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %v, want an error naming the project and reporting HTTP 400", err)
	}
	if got := meta.GetExternalName(cr); got != "" {
		t.Errorf("external name = %q, want it unset", got)
	}
}

func TestMissingSecretIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK}
	_, err := connect(t, api).Create(t.Context(), gitSettings(""))
	if !kerrors.IsNotFound(err) {
		t.Errorf("error = %v, want the Secret not found", err)
	}
	assertReceived(t, api)
}

func TestMissingSecretKeyIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK}
	s := keySecret(privateKey)
	s.Data = map[string][]byte{"other": []byte(privateKey)}
	_, err := connect(t, api, s).Create(t.Context(), gitSettings(""))
	if err == nil || !strings.Contains(err.Error(), `key "privateKey"`) {
		t.Errorf("error = %v, want an error naming the missing key", err)
	}
	assertReceived(t, api)
}

func TestMalformedProjectIDIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK, response: remoteGit}
	cr := gitSettings("")
	cr.Spec.ForProvider.ProjectID = "not-a-uuid"
	if _, err := connect(t, api, keySecret(privateKey)).Create(t.Context(), cr); err == nil || !strings.Contains(err.Error(), "not-a-uuid") {
		t.Errorf("create error = %v, want an error naming the malformed project id", err)
	}
	if _, err := connect(t, api, keySecret(privateKey)).Observe(t.Context(), gitSettings("not-a-uuid")); err == nil || !strings.Contains(err.Error(), "not-a-uuid") {
		t.Errorf("observe error = %v, want an error naming the malformed external name", err)
	}
	if _, err := connect(t, api, keySecret(privateKey)).Update(t.Context(), gitSettings("not-a-uuid")); err == nil || !strings.Contains(err.Error(), "not-a-uuid") {
		t.Errorf("update error = %v, want an error naming the malformed external name", err)
	}
	assertReceived(t, api)
}

func TestUpdatePostsSettingsAndRecordsHash(t *testing.T) {
	api := &fakeAPI{status: http.StatusOK}
	cr := gitSettings(projectID)
	if _, err := connect(t, api, keySecret(privateKey)).Update(t.Context(), cr); err != nil {
		t.Fatal(err)
	}
	if cr.Status.AtProvider.PrivateKeyHash != hash(privateKey) {
		t.Errorf("atProvider.privateKeyHash = %q, want %q", cr.Status.AtProvider.PrivateKeyHash, hash(privateKey))
	}
	assertReceived(t, api, wantPost)
}

func TestUpdateServerErrorIsAnError(t *testing.T) {
	api := &fakeAPI{status: http.StatusInternalServerError, response: `{"message":"boom"}`}
	cr := gitSettings(projectID)
	_, err := connect(t, api, keySecret(privateKey)).Update(t.Context(), cr)
	if err == nil || !strings.Contains(err.Error(), projectID) || !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %v, want an error naming the project and reporting HTTP 500", err)
	}
	if cr.Status.AtProvider.PrivateKeyHash != "" {
		t.Errorf("atProvider.privateKeyHash = %q, want it unset after a failed update", cr.Status.AtProvider.PrivateKeyHash)
	}
}

func unreachable(t *testing.T) managed.TypedExternalClient[*v1alpha1.ProjectGitSettings] {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	c, err := projects.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &external{kube: kfake.NewClientBuilder().WithObjects(keySecret(privateKey)).Build(), client: c}
}

func TestUnreachableAPIIsAnError(t *testing.T) {
	if _, err := unreachable(t).Observe(t.Context(), applied(projectID)); err == nil {
		t.Error("observe error = nil, want a transport error")
	}
	if _, err := unreachable(t).Create(t.Context(), gitSettings("")); err == nil {
		t.Error("create error = nil, want a transport error")
	}
	if _, err := unreachable(t).Update(t.Context(), gitSettings(projectID)); err == nil {
		t.Error("update error = nil, want a transport error")
	}
}

func TestDeleteMakesNoRequest(t *testing.T) {
	api := &fakeAPI{status: http.StatusInternalServerError}
	if _, err := connect(t, api).Delete(t.Context(), applied(projectID)); err != nil {
		t.Fatal(err)
	}
	assertReceived(t, api)
}
