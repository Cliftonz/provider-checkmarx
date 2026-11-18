// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const tokenRefreshMargin = 30 * time.Second

var now = time.Now

var exchanges singleflight.Group

var tokens = struct {
	sync.Mutex
	cache map[[sha256.Size]byte]cachedToken
}{cache: map[[sha256.Size]byte]cachedToken{}}

type cachedToken struct {
	value   string
	expires time.Time
}

type bearerTransport struct {
	tokenURL string
	form     url.Values
	key      [sha256.Size]byte
}

func newBearerTransport(creds credentials) (*bearerTransport, error) {
	var form url.Values
	if creds.APIKey != "" {
		clientID, err := clientIDFromAPIKey(creds.APIKey)
		if err != nil {
			return nil, err
		}
		form = url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "refresh_token": {creds.APIKey}}
	} else {
		form = url.Values{"grant_type": {"client_credentials"}, "client_id": {creds.ClientID}, "client_secret": {creds.ClientSecret}}
	}
	return &bearerTransport{
		tokenURL: creds.IAMURL + "/auth/realms/" + url.PathEscape(creds.Tenant) + "/protocol/openid-connect/token",
		form:     form,
		key:      sha256.Sum256([]byte(strings.Join([]string{creds.IAMURL, creds.Tenant, creds.APIKey, creds.ClientID, creds.ClientSecret}, "\x00"))),
	}, nil
}

func clientIDFromAPIKey(apiKey string) (string, error) {
	parts := strings.Split(apiKey, ".")
	if len(parts) != 3 {
		return "", ErrMalformedAPIKey
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ErrMalformedAPIKey
	}
	var claims struct {
		Azp *string `json:"azp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", ErrMalformedAPIKey
	}
	switch {
	case claims.Azp == nil:
		return "ast-app", nil
	case *claims.Azp == "":
		return "", ErrMalformedAPIKey
	}
	return *claims.Azp, nil
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.token(req.Context())
	if err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, err
	}
	resp, err := send(req, token)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}
	t.evict(token)
	if req.Body != nil && req.GetBody == nil {
		return resp, nil
	}
	_ = resp.Body.Close()
	if token, err = t.token(req.Context()); err != nil {
		return nil, err
	}
	retry := *req
	if req.GetBody != nil {
		retry.Body, _ = req.GetBody()
	}
	resp, err = send(&retry, token)
	if err == nil && resp.StatusCode == http.StatusUnauthorized {
		t.evict(token)
	}
	return resp, err
}

func (t *bearerTransport) evict(token string) {
	tokens.Lock()
	if tokens.cache[t.key].value == token {
		delete(tokens.cache, t.key)
	}
	tokens.Unlock()
}

func send(req *http.Request, token string) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+token)
	return http.DefaultTransport.RoundTrip(r)
}

func (t *bearerTransport) token(ctx context.Context) (string, error) {
	if token, ok := t.freshToken(); ok {
		return token, nil
	}
	token, err, _ := exchanges.Do(string(t.key[:]), func() (any, error) {
		if token, ok := t.freshToken(); ok {
			return token, nil
		}
		return t.exchange(ctx)
	})
	return token.(string), err
}

func (t *bearerTransport) freshToken() (string, bool) {
	tokens.Lock()
	cached := tokens.cache[t.key]
	tokens.Unlock()
	return cached.value, cached.expires.Sub(now()) >= tokenRefreshMargin
}

func (t *bearerTransport) exchange(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, t.tokenURL, strings.NewReader(t.form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request returned %d: %s", resp.StatusCode, body)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return "", err
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("token response from %s: access_token is missing", t.tokenURL)
	}
	tokens.Lock()
	for key, cached := range tokens.cache {
		if cached.expires.Before(now()) {
			delete(tokens.cache, key)
		}
	}
	if token.ExpiresIn > 0 {
		tokens.cache[t.key] = cachedToken{value: token.AccessToken, expires: now().Add(time.Duration(token.ExpiresIn) * time.Second)}
	}
	tokens.Unlock()
	return token.AccessToken, nil
}
