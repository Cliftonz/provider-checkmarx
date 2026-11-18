// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package kit

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/Cliftonz/provider-checkmarx/apis/v1beta1"
	"github.com/Cliftonz/provider-checkmarx/internal/clients"
)

var kinds atomic.Int64

func kindFor(t *testing.T) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "test.checkmarx.crossplane.io", Version: "v1alpha1", Kind: fmt.Sprintf("%s%d", strings.ReplaceAll(t.Name(), "/", ""), kinds.Add(1))}
}

func newManager(t *testing.T, gvk schema.GroupVersionKind) (manager.Manager, *bytes.Buffer) {
	t.Helper()
	s := runtime.NewScheme()
	s.AddKnownTypeWithName(gvk, &fake.ModernManaged{})
	var logs bytes.Buffer
	mgr, err := manager.New(&rest.Config{Host: "http://127.0.0.1:1"}, manager.Options{
		Scheme:                 s,
		Logger:                 zap.New(zap.WriteTo(&logs)),
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		t.Fatal(err)
	}
	return mgr, &logs
}

func options(gate controller.Gate) controller.Options {
	o := controller.DefaultOptions()
	o.Logger = logging.NewNopLogger()
	o.MetricOptions = &controller.MetricOptions{}
	o.Gate = gate
	return o
}

func noExternal(*clients.Connection) managed.TypedExternalClient[*fake.ModernManaged] {
	return nil
}

func TestSetupRegistersControllerNamedForKind(t *testing.T) {
	gvk := kindFor(t)
	mgr, _ := newManager(t, gvk)

	if err := Setup(mgr, options(nil), gvk, &fake.ModernManaged{}, nil, noExternal); err != nil {
		t.Fatalf("Setup() unexpected error: %v", err)
	}
	err := Setup(mgr, options(nil), gvk, &fake.ModernManaged{}, nil, noExternal)
	if want := managed.ControllerName(gvk.GroupKind().String()); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("second Setup() error = %v, want a duplicate controller %q", err, want)
	}
}

type firstAddFails struct {
	manager.Manager
	err error
}

func (m *firstAddFails) Add(r manager.Runnable) error {
	if err := m.err; err != nil {
		m.err = nil
		return err
	}
	return m.Manager.Add(r)
}

func TestSetupReturnsStateRecorderAddError(t *testing.T) {
	gvk := kindFor(t)
	mgr, _ := newManager(t, gvk)
	boom := errors.New("boom")

	if err := Setup(&firstAddFails{Manager: mgr, err: boom}, options(nil), gvk, &fake.ModernManaged{}, nil, noExternal); !errors.Is(err, boom) {
		t.Fatalf("Setup() error = %v, want %v", err, boom)
	}
}

type recordingGate struct {
	controller.Gate
	callbacks []func()
	gvks      []schema.GroupVersionKind
}

func (g *recordingGate) Register(callback func(), gvks ...schema.GroupVersionKind) {
	g.callbacks = append(g.callbacks, callback)
	g.gvks = append(g.gvks, gvks...)
}

func (g *recordingGate) open() {
	for _, cb := range g.callbacks {
		cb()
	}
}

func TestSetupGatedDefersSetupUntilKindIsReady(t *testing.T) {
	gvk := kindFor(t)
	mgr, logs := newManager(t, gvk)
	gate := &recordingGate{}

	if err := SetupGated(mgr, options(gate), gvk, &fake.ModernManaged{}, nil, noExternal); err != nil {
		t.Fatalf("SetupGated() unexpected error: %v", err)
	}
	if len(gate.gvks) != 1 || gate.gvks[0] != gvk {
		t.Fatalf("gate registered for %v, want [%v]", gate.gvks, gvk)
	}
	gate.open()

	if logs.Len() != 0 {
		t.Fatalf("logs = %q, want none", logs)
	}
	err := Setup(mgr, options(nil), gvk, &fake.ModernManaged{}, nil, noExternal)
	if want := managed.ControllerName(gvk.GroupKind().String()); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Setup() after the gate opened error = %v, want a duplicate controller %q", err, want)
	}
}

func TestSetupGatedLogsSetupError(t *testing.T) {
	gvk := kindFor(t)
	mgr, logs := newManager(t, gvk)
	gate := &recordingGate{}
	if err := Setup(mgr, options(nil), gvk, &fake.ModernManaged{}, nil, noExternal); err != nil {
		t.Fatalf("Setup() unexpected error: %v", err)
	}

	if err := SetupGated(mgr, options(gate), gvk, &fake.ModernManaged{}, nil, noExternal); err != nil {
		t.Fatalf("SetupGated() unexpected error: %v", err)
	}
	gate.open()

	for _, want := range []string{"unable to setup reconciler", gvk.String(), managed.ControllerName(gvk.GroupKind().String())} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("logs = %q, want them to contain %q", logs, want)
		}
	}
}

func fakeCheckmarx(t *testing.T) (iam, api *httptest.Server, authorization *string) {
	t.Helper()
	iam = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"access_token":"issued","expires_in":300}`)
	}))
	t.Cleanup(iam.Close)
	authorization = new(string)
	api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*authorization = r.Header.Get("Authorization")
	}))
	t.Cleanup(api.Close)
	return iam, api, authorization
}

func kubeWithProviderConfig(t *testing.T, credentials string) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := v1beta1.SchemeBuilder.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	pc := &v1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "team-a"},
		Spec: v1beta1.ProviderConfigSpec{Credentials: v1beta1.ProviderCredentials{
			Source: xpv2.CredentialsSourceSecret,
			CommonCredentialSelectors: xpv2.CommonCredentialSelectors{SecretRef: &xpv2.SecretKeySelector{
				SecretReference: xpv2.SecretReference{Name: "checkmarx-creds"},
				Key:             "credentials",
			}},
		}},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "checkmarx-creds", Namespace: "team-a"},
		Data:       map[string][]byte{"credentials": []byte(credentials)},
	}
	return kfake.NewClientBuilder().WithScheme(s).WithObjects(pc, secret).Build()
}

func managedWithConfig(kind string) *fake.ModernManaged {
	mg := &fake.ModernManaged{ObjectMeta: metav1.ObjectMeta{Name: "my-project", Namespace: "team-a", UID: "mg-uid"}}
	mg.SetProviderConfigReference(&xpv2.ProviderConfigReference{Kind: kind, Name: "default"})
	return mg
}

func TestConnectHandsConnectionToExternal(t *testing.T) {
	iam, api, authorization := fakeCheckmarx(t)
	tenant := strings.ReplaceAll(t.Name(), "/", "-")
	kube := kubeWithProviderConfig(t, fmt.Sprintf(`{"baseUrl":%q,"iamUrl":%q,"tenant":%q,"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`, api.URL, iam.URL, tenant))
	var got *clients.Connection
	newExternal := func(c *clients.Connection) managed.TypedExternalClient[*fake.ModernManaged] {
		got = c
		return nil
	}

	if _, err := connector(kube, newExternal).Connect(t.Context(), managedWithConfig(v1beta1.ProviderConfigKind)); err != nil {
		t.Fatalf("Connect() unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("NewExternal was not handed a Connection")
	}
	if got.BaseURL != api.URL || got.IAMURL != iam.URL || got.Tenant != tenant {
		t.Fatalf("Connection = %+v, want baseUrl %s, iamUrl %s, tenant %s", got, api.URL, iam.URL, tenant)
	}
	resp, err := got.HTTPClient.Get(got.BaseURL + "/api/projects")
	if err != nil {
		t.Fatalf("GET unexpected error: %v", err)
	}
	_ = resp.Body.Close()
	if *authorization != "Bearer issued" {
		t.Fatalf("Authorization = %q, want Bearer issued", *authorization)
	}
}

func TestConnectReturnsConnectionError(t *testing.T) {
	kube := kubeWithProviderConfig(t, `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme","apiKey":"opaque-key"}`)
	_, err := connector(kube, noExternal).Connect(t.Context(), managedWithConfig(v1beta1.ProviderConfigKind))

	if !errors.Is(err, clients.ErrMalformedAPIKey) {
		t.Fatalf("Connect() error = %v, want %v", err, clients.ErrMalformedAPIKey)
	}
}

type fakeResponse struct {
	code int
	body string
}

func (r fakeResponse) StatusCode() int { return r.code }
func (r fakeResponse) Status() string  { return fmt.Sprintf("%d %s", r.code, http.StatusText(r.code)) }
func (r fakeResponse) GetBody() []byte { return []byte(r.body) }

func TestCheckStatusAcceptsSuccess(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusCreated, http.StatusNoContent} {
		if err := CheckStatus("get project", fakeResponse{code: code}); err != nil {
			t.Errorf("CheckStatus(%d) = %v, want nil", code, err)
		}
	}
}

func TestCheckStatusReportsActionStatusAndBody(t *testing.T) {
	for _, code := range []int{http.StatusSwitchingProtocols, http.StatusMultipleChoices, http.StatusBadRequest, http.StatusNotFound, http.StatusTooManyRequests, http.StatusInternalServerError} {
		r := fakeResponse{code: code, body: `{"message":"nope"}`}
		err := CheckStatus("get project", r)
		if err == nil {
			t.Fatalf("CheckStatus(%d) = nil, want error", code)
		}
		for _, want := range []string{"get project", r.Status(), r.body} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("CheckStatus(%d) = %q, want it to contain %q", code, err, want)
			}
		}
	}
}

func TestNotFound(t *testing.T) {
	for code, want := range map[int]bool{http.StatusNotFound: true, http.StatusOK: false, http.StatusGone: false, http.StatusInternalServerError: false} {
		if got := NotFound(fakeResponse{code: code}); got != want {
			t.Errorf("NotFound(%d) = %v, want %v", code, got, want)
		}
	}
}

func TestValueOfNilIsZero(t *testing.T) {
	if got := Value[string](nil); got != "" {
		t.Errorf("Value(nil) = %q, want empty", got)
	}
}

func TestValueOfPointerIsPointee(t *testing.T) {
	s := "x"
	if got := Value(&s); got != "x" {
		t.Errorf("Value(&x) = %q, want x", got)
	}
}

func TestTimeOfNilIsNil(t *testing.T) {
	if got := Time(nil); got != nil {
		t.Errorf("Time(nil) = %v, want nil", got)
	}
}

func TestTimeOfValueIsSameInstant(t *testing.T) {
	at := time.Date(2026, 11, 18, 9, 30, 0, 0, time.UTC)
	if got := Time(&at); got == nil || !got.Time.Equal(at) {
		t.Errorf("Time(%v) = %v, want %v", at, got, at)
	}
}

func managedNamed(externalName string) *fake.ModernManaged {
	mg := &fake.ModernManaged{}
	meta.SetExternalName(mg, externalName)
	return mg
}

func TestExternalUUIDParsesExternalName(t *testing.T) {
	want := uuid.MustParse("3f2c1b7e-9a4d-4c2e-8f1a-6b5d4e3c2a10")
	got, err := ExternalUUID(managedNamed(want.String()))
	if err != nil {
		t.Fatalf("ExternalUUID() unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("ExternalUUID() = %v, want %v", got, want)
	}
}

func TestExternalUUIDNamesMalformedExternalName(t *testing.T) {
	_, err := ExternalUUID(managedNamed("not-a-uuid"))
	if err == nil || !strings.Contains(err.Error(), `"not-a-uuid"`) {
		t.Fatalf("ExternalUUID() error = %v, want it to name %q", err, "not-a-uuid")
	}
}
