// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Cliftonz/provider-checkmarx/apis/v1beta1"
)

type tokenRequest struct {
	method      string
	path        string
	contentType string
	form        url.Values
}

type fakeIAM struct {
	mu                               sync.Mutex
	delay                            time.Duration
	server                           *httptest.Server
	requests                         []tokenRequest
	tokenResponseStatus              int
	tokenResponseBody                string
	successfulResponsesBeforeFailure int
}

const fakeTokenExpiresIn = 300

func newFakeIAM(t *testing.T) *fakeIAM {
	t.Helper()
	iam := &fakeIAM{}
	iam.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(iam.delay)
		iam.mu.Lock()
		defer iam.mu.Unlock()
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		iam.requests = append(iam.requests, tokenRequest{method: r.Method, path: r.URL.EscapedPath(), contentType: r.Header.Get("Content-Type"), form: r.PostForm})
		if iam.tokenResponseStatus != 0 && len(iam.requests) > iam.successfulResponsesBeforeFailure {
			w.WriteHeader(iam.tokenResponseStatus)
			_, _ = io.WriteString(w, iam.tokenResponseBody)
			return
		}
		_, _ = fmt.Fprintf(w, `{"access_token":"token-%d","expires_in":%d}`, len(iam.requests), fakeTokenExpiresIn)
	}))
	t.Cleanup(iam.server.Close)
	return iam
}

type apiRequest struct {
	authorization string
	body          string
}

type fakeAPI struct {
	rejectedAuthorization string
	holdFirst             chan struct{}
	dropConnectionsFrom   int32
	arrivals              atomic.Int32
	mu                    sync.Mutex
	server                *httptest.Server
	requests              []apiRequest
	statuses              []int
}

func newFakeAPI(t *testing.T, statuses ...int) *fakeAPI {
	t.Helper()
	api := &fakeAPI{statuses: statuses}
	api.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrival := api.arrivals.Add(1)
		if arrival == 1 && api.holdFirst != nil {
			<-api.holdFirst
		}
		if api.dropConnectionsFrom != 0 && arrival >= api.dropConnectionsFrom {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
			return
		}
		api.mu.Lock()
		defer api.mu.Unlock()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		api.requests = append(api.requests, apiRequest{authorization: r.Header.Get("Authorization"), body: string(body)})
		status := http.StatusOK
		if len(api.requests) <= len(api.statuses) {
			status = api.statuses[len(api.requests)-1]
		}
		if api.rejectedAuthorization != "" && r.Header.Get("Authorization") == api.rejectedAuthorization {
			status = http.StatusUnauthorized
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(api.server.Close)
	return api
}

func tenantFor(t *testing.T) string {
	return strings.ReplaceAll(t.Name(), "/", "-")
}

func connect(t *testing.T, iam *fakeIAM, api *fakeAPI, credentialFields string) *Connection {
	t.Helper()
	credentials := fmt.Sprintf(`{"baseUrl":%q,"iamUrl":%q,"tenant":%q,%s}`, api.server.URL, iam.server.URL, tenantFor(t), credentialFields)
	kube := kubeClient(t, namespacedProviderConfig(), namespace, credentials)
	conn, err := NewClient(context.Background(), kube, managed(v1beta1.ProviderConfigKind))
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}
	return conn
}

func get(t *testing.T, conn *Connection) *http.Response {
	t.Helper()
	resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
	if err != nil {
		t.Fatalf("GET unexpected error: %v", err)
	}
	_ = resp.Body.Close()
	return resp
}

func assertTokenRequest(t *testing.T, got tokenRequest, wantForm url.Values) {
	t.Helper()
	wantPath := "/auth/realms/" + tenantFor(t) + "/protocol/openid-connect/token"
	if got.method != http.MethodPost || got.path != wantPath || got.contentType != "application/x-www-form-urlencoded" {
		t.Errorf("token request = %s %s (%s), want POST %s (application/x-www-form-urlencoded)", got.method, got.path, got.contentType, wantPath)
	}
	if got.form.Encode() != wantForm.Encode() {
		t.Errorf("token form = %s, want %s", got.form.Encode(), wantForm.Encode())
	}
}

func TestTransportExchangesAPIKeyWithAzpClientID(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	payload, err := json.Marshal(map[string]string{"azp": "my-client"})
	if err != nil {
		t.Fatal(err)
	}
	apiKey := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	conn := connect(t, iam, api, fmt.Sprintf(`"apiKey":%q`, apiKey))

	get(t, conn)

	if len(iam.requests) != 1 {
		t.Fatalf("token requests = %d, want 1", len(iam.requests))
	}
	assertTokenRequest(t, iam.requests[0], url.Values{"grant_type": {"refresh_token"}, "client_id": {"my-client"}, "refresh_token": {apiKey}})
}

func TestTransportExchangesAPIKeyWithDefaultClientID(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	apiKey := "header." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user"}`)) + ".signature"
	conn := connect(t, iam, api, fmt.Sprintf(`"apiKey":%q`, apiKey))

	get(t, conn)

	if len(iam.requests) != 1 {
		t.Fatalf("token requests = %d, want 1", len(iam.requests))
	}
	assertTokenRequest(t, iam.requests[0], url.Values{"grant_type": {"refresh_token"}, "client_id": {"ast-app"}, "refresh_token": {apiKey}})
}

func TestTransportExchangesClientCredentials(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	conn := connect(t, iam, api, `"clientId":"my client","clientSecret":"s&cret=1"`)

	get(t, conn)

	if len(iam.requests) != 1 {
		t.Fatalf("token requests = %d, want 1", len(iam.requests))
	}
	assertTokenRequest(t, iam.requests[0], url.Values{"grant_type": {"client_credentials"}, "client_id": {"my client"}, "client_secret": {"s&cret=1"}})
}

func TestTransportSendsBearerOnRequests(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	get(t, conn)

	if len(api.requests) != 1 || api.requests[0].authorization != "Bearer token-1" {
		t.Fatalf("API requests = %+v, want one with Authorization Bearer token-1", api.requests)
	}
}

func freezeClock(t *testing.T) *time.Time {
	t.Helper()
	current := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	original := now
	now = func() time.Time { return current }
	t.Cleanup(func() { now = original })
	return &current
}

func TestTransportReusesTokenUntilThirtySecondsBeforeExpiry(t *testing.T) {
	clock := freezeClock(t)
	iam, api := newFakeIAM(t), newFakeAPI(t)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	get(t, conn)
	*clock = clock.Add(270 * time.Second)
	get(t, conn)

	if len(iam.requests) != 1 {
		t.Fatalf("token requests = %d, want 1", len(iam.requests))
	}
	if api.requests[1].authorization != "Bearer token-1" {
		t.Fatalf("second Authorization = %q, want Bearer token-1", api.requests[1].authorization)
	}
}

func TestTransportRefreshesWhenFewerThanThirtySecondsRemain(t *testing.T) {
	clock := freezeClock(t)
	iam, api := newFakeIAM(t), newFakeAPI(t)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	get(t, conn)
	*clock = clock.Add(271 * time.Second)
	get(t, conn)

	if len(iam.requests) != 2 {
		t.Fatalf("token requests = %d, want 2", len(iam.requests))
	}
	if api.requests[1].authorization != "Bearer token-2" {
		t.Fatalf("second Authorization = %q, want Bearer token-2", api.requests[1].authorization)
	}
}

func TestTransportCachesTokenPerCredentialSet(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)

	get(t, connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`))
	get(t, connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`))
	get(t, connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJvdGhlciJ9.s"`))

	if len(iam.requests) != 2 {
		t.Fatalf("token requests = %d, want 2", len(iam.requests))
	}
	if got := []string{api.requests[0].authorization, api.requests[1].authorization, api.requests[2].authorization}; got[0] != "Bearer token-1" || got[1] != "Bearer token-1" || got[2] != "Bearer token-2" {
		t.Fatalf("Authorization headers = %v, want token-1, token-1, token-2", got)
	}
}

func TestTransportSurfacesTokenEndpointError(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	iam.tokenResponseStatus, iam.tokenResponseBody = http.StatusUnauthorized, `{"error":"invalid_grant"}`
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("GET error = nil, want the token endpoint error")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), `{"error":"invalid_grant"}`) {
		t.Fatalf("GET error = %v, want status 401 and the response body", err)
	}
	if len(api.requests) != 0 {
		t.Fatalf("API requests = %d, want 0", len(api.requests))
	}
}

func post(t *testing.T, conn *Connection, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, conn.BaseURL+"/api/projects", body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := conn.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("POST unexpected error: %v", err)
	}
	_ = resp.Body.Close()
	return resp
}

func TestTransportRetriesOnceAfter401(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t, http.StatusUnauthorized, http.StatusCreated)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	resp := post(t, conn, strings.NewReader(`{"name":"p"}`))

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if len(iam.requests) != 2 {
		t.Fatalf("token requests = %d, want 2", len(iam.requests))
	}
	want := []apiRequest{{authorization: "Bearer token-1", body: `{"name":"p"}`}, {authorization: "Bearer token-2", body: `{"name":"p"}`}}
	if len(api.requests) != 2 || api.requests[0] != want[0] || api.requests[1] != want[1] {
		t.Fatalf("API requests = %+v, want %+v", api.requests, want)
	}
}

func TestTransportRetriesBodylessRequestAfter401(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t, http.StatusUnauthorized)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	resp := get(t, conn)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if len(api.requests) != 2 || api.requests[1].authorization != "Bearer token-2" {
		t.Fatalf("API requests = %+v, want a retry with Bearer token-2", api.requests)
	}
}

func TestTransportReturnsSecond401(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusOK)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	resp := get(t, conn)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if len(api.requests) != 2 {
		t.Fatalf("API requests = %d, want 2", len(api.requests))
	}
}

func TestTransportDoesNotRetryUnreplayableBody(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t, http.StatusUnauthorized, http.StatusCreated)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	resp := post(t, conn, struct{ io.Reader }{strings.NewReader(`{"name":"p"}`)})

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if len(api.requests) != 1 || len(iam.requests) != 1 {
		t.Fatalf("API requests = %d, token requests = %d, want 1 and 1", len(api.requests), len(iam.requests))
	}
}

func TestTransportPassesThrough429(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t, http.StatusTooManyRequests, http.StatusOK)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	resp := get(t, conn)

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	if len(api.requests) != 1 || len(iam.requests) != 1 {
		t.Fatalf("API requests = %d, token requests = %d, want 1 and 1", len(api.requests), len(iam.requests))
	}
}

func TestTransportSurfacesReexchangeErrorAfter401(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t, http.StatusUnauthorized)
	iam.tokenResponseStatus, iam.tokenResponseBody, iam.successfulResponsesBeforeFailure = http.StatusServiceUnavailable, "iam down", 1
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("GET error = nil, want the token endpoint error")
	}
	if !strings.Contains(err.Error(), "503") || !strings.Contains(err.Error(), "iam down") {
		t.Fatalf("GET error = %v, want status 503 and the response body", err)
	}
	if len(api.requests) != 1 {
		t.Fatalf("API requests = %+v, want only the first request", api.requests)
	}
}

func TestTransportSurfacesMalformedTokenResponse(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	iam.tokenResponseStatus, iam.tokenResponseBody = http.StatusOK, "not json"
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("GET error = nil, want the token response decode error")
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("GET error = %v, want a JSON syntax error", err)
	}
	if len(api.requests) != 0 {
		t.Fatalf("API requests = %d, want 0", len(api.requests))
	}
}

func TestTransportSurfacesUnreachableIAM(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)
	iam.server.Close()

	resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("GET error = nil, want the IAM transport error")
	}
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("GET error = %v, want a network error", err)
	}
	if len(api.requests) != 0 {
		t.Fatalf("API requests = %d, want 0", len(api.requests))
	}
}

func TestTransportRejectsTokenResponseWithoutAccessToken(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	iam.tokenResponseStatus, iam.tokenResponseBody = http.StatusOK, `{"expires_in":3600}`
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("GET error = nil, want a missing access_token error")
	}
	if !strings.Contains(err.Error(), "/protocol/openid-connect/token") || !strings.Contains(err.Error(), "access_token is missing") {
		t.Fatalf("GET error = %v, want the token endpoint and access_token is missing", err)
	}
	if len(api.requests) != 0 {
		t.Fatalf("API requests = %d, want 0", len(api.requests))
	}
}

func TestTransportCancelsExchangeWithRequest(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, conn.BaseURL+"/api/projects", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := conn.HTTPClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("GET error = nil, want context.Canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GET error = %v, want %v", err, context.Canceled)
	}
	if len(iam.requests) != 0 {
		t.Fatalf("token requests = %d, want 0", len(iam.requests))
	}
}

func TestTransportEscapesTenantInTokenPath(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	credentials := fmt.Sprintf(`{"baseUrl":%q,"iamUrl":%q,"tenant":"a/b","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`, api.server.URL, iam.server.URL)
	kube := kubeClient(t, namespacedProviderConfig(), namespace, credentials)
	conn, err := NewClient(context.Background(), kube, managed(v1beta1.ProviderConfigKind))
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	get(t, conn)

	if len(iam.requests) != 1 || iam.requests[0].path != "/auth/realms/a%2Fb/protocol/openid-connect/token" {
		t.Fatalf("token requests = %+v, want one to /auth/realms/a%%2Fb/protocol/openid-connect/token", iam.requests)
	}
}

func TestTransportExchangesOnceForConcurrentRequests(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	iam.delay = 200 * time.Millisecond
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
			if err != nil {
				t.Error(err)
				return
			}
			_ = resp.Body.Close()
		})
	}
	wg.Wait()

	if len(iam.requests) != 1 {
		t.Fatalf("token requests = %d, want 1", len(iam.requests))
	}
}

func TestTransportPrunesExpiredTokensOnExchange(t *testing.T) {
	clock := freezeClock(t)
	iam, api := newFakeIAM(t), newFakeAPI(t)
	get(t, connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`))
	rotated := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJvdGhlciJ9.s"`)

	*clock = clock.Add(301 * time.Second)
	get(t, rotated)

	tokens.Lock()
	defer tokens.Unlock()
	for _, cached := range tokens.cache {
		if cached.expires.Before(*clock) {
			t.Fatalf("token cache holds an entry that expired at %v, before %v", cached.expires, *clock)
		}
	}
}

type closeRecorder struct {
	io.Reader
	closed bool
}

func (c *closeRecorder) Close() error {
	c.closed = true
	return nil
}

func TestTransportClosesRequestBodyWhenTokenExchangeFails(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	iam.tokenResponseStatus, iam.tokenResponseBody = http.StatusInternalServerError, "boom"
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)
	body := &closeRecorder{Reader: strings.NewReader(`{"name":"p"}`)}
	req, err := http.NewRequest(http.MethodPost, conn.BaseURL+"/api/projects", body)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := conn.HTTPClient.Transport.RoundTrip(req); err == nil {
		t.Fatal("RoundTrip() error = nil, want the token endpoint error")
	}
	if !body.closed {
		t.Fatal("request body was not closed")
	}
}

func TestTransportEvictsRejectedTokenWhenReexchangeFails(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t, http.StatusUnauthorized)
	iam.tokenResponseStatus, iam.tokenResponseBody, iam.successfulResponsesBeforeFailure = http.StatusServiceUnavailable, "iam down", 1
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	if _, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects"); err == nil {
		t.Fatal("first GET error = nil, want the re-exchange error")
	}
	if len(iam.requests) != 2 {
		t.Fatalf("token requests after the failed re-exchange = %d, want 2", len(iam.requests))
	}
	if _, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects"); err == nil {
		t.Fatal("second GET error = nil, want the token endpoint error")
	}
	if len(iam.requests) != 3 {
		t.Fatalf("token requests = %d, want 3", len(iam.requests))
	}
}

func TestTransportReexchangesOnceForConcurrent401s(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	iam.delay = 200 * time.Millisecond
	api.rejectedAuthorization = "Bearer token-1"
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
			if err != nil {
				t.Error(err)
				return
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
		})
	}
	wg.Wait()

	if len(iam.requests) != 2 {
		t.Fatalf("token requests = %d, want 2", len(iam.requests))
	}
}

func TestTransportDoesNotExchangeAgainWhenAnotherRequestJustRefreshed(t *testing.T) {
	current := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	var pauseNextRead atomic.Bool
	resume := make(chan struct{})
	original := now
	now = func() time.Time {
		if pauseNextRead.CompareAndSwap(true, false) {
			<-resume
		}
		return current
	}
	t.Cleanup(func() { now = original })
	iam, api := newFakeIAM(t), newFakeAPI(t)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	pauseNextRead.Store(true)
	stale := make(chan *http.Response)
	go func() {
		resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
		if err != nil {
			t.Error(err)
		}
		stale <- resp
	}()
	for pauseNextRead.Load() {
		runtime.Gosched()
	}
	get(t, conn)
	close(resume)
	if resp := <-stale; resp != nil {
		_ = resp.Body.Close()
	}

	if len(iam.requests) != 1 {
		t.Fatalf("token requests = %d, want 1", len(iam.requests))
	}
}

func TestTransportKeepsRefreshedTokenWhenAStale401ArrivesLate(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	api.rejectedAuthorization = "Bearer token-1"
	api.holdFirst = make(chan struct{})
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	late := make(chan *http.Response)
	go func() {
		resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
		if err != nil {
			t.Error(err)
		}
		late <- resp
	}()
	for api.arrivals.Load() == 0 {
		runtime.Gosched()
	}
	get(t, conn)
	close(api.holdFirst)
	if resp := <-late; resp != nil {
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("late request status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	}

	if len(iam.requests) != 2 {
		t.Fatalf("token requests = %d, want 2", len(iam.requests))
	}
}

func TestTransportSurfacesAPITransportError(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)
	api.server.Close()

	resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("GET error = nil, want the API transport error")
	}
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("GET error = %v, want a network error", err)
	}
}

func TestTransportDoesNotCacheTokenWithoutPositiveExpiresIn(t *testing.T) {
	cases := map[string]string{
		"Missing":  `{"access_token":"uncached-missing"}`,
		"Zero":     `{"access_token":"uncached-zero","expires_in":0}`,
		"Negative": `{"access_token":"uncached-negative","expires_in":-5}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			iam, api := newFakeIAM(t), newFakeAPI(t)
			iam.tokenResponseStatus, iam.tokenResponseBody = http.StatusOK, body
			conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

			get(t, conn)
			get(t, conn)

			if len(iam.requests) != 2 {
				t.Fatalf("token requests = %d, want 2", len(iam.requests))
			}
			tokens.Lock()
			defer tokens.Unlock()
			for _, cached := range tokens.cache {
				if strings.HasPrefix(cached.value, "uncached-") {
					t.Fatalf("token cache holds %q, want no entry", cached.value)
				}
			}
		})
	}
}

func TestTransportEvictsRejectedTokenWhenBodyIsUnreplayable(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t, http.StatusUnauthorized)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	post(t, conn, struct{ io.Reader }{strings.NewReader(`{"name":"p"}`)})
	get(t, conn)

	if len(iam.requests) != 2 {
		t.Fatalf("token requests = %d, want 2", len(iam.requests))
	}
}

func TestTransportEvictsTokenRejectedOnRetry(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t, http.StatusUnauthorized, http.StatusUnauthorized)
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	get(t, conn)
	get(t, conn)

	if len(iam.requests) != 3 {
		t.Fatalf("token requests = %d, want 3", len(iam.requests))
	}
}

func TestTransportSurfacesTransportErrorOnRetry(t *testing.T) {
	iam, api := newFakeIAM(t), newFakeAPI(t, http.StatusUnauthorized)
	api.dropConnectionsFrom = 2
	conn := connect(t, iam, api, `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"`)

	resp, err := conn.HTTPClient.Get(conn.BaseURL + "/api/projects")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("GET error = nil, want the retry's transport error")
	}
	if len(iam.requests) != 2 {
		t.Fatalf("token requests = %d, want 2", len(iam.requests))
	}
}

func TestTransportCachesClientCredentialsTokensPerClient(t *testing.T) {
	cases := map[string][2]string{
		"DifferentClientID":     {`"clientId":"one","clientSecret":"s"`, `"clientId":"two","clientSecret":"s"`},
		"DifferentClientSecret": {`"clientId":"one","clientSecret":"s1"`, `"clientId":"one","clientSecret":"s2"`},
	}
	for name, credentials := range cases {
		t.Run(name, func(t *testing.T) {
			iam, api := newFakeIAM(t), newFakeAPI(t)

			get(t, connect(t, iam, api, credentials[0]))
			get(t, connect(t, iam, api, credentials[1]))

			if len(iam.requests) != 2 {
				t.Fatalf("token requests = %d, want 2", len(iam.requests))
			}
		})
	}
}
