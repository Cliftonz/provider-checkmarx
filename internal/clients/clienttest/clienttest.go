// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package clienttest

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
)

type Request struct {
	Method   string
	Path     string
	Query    url.Values
	BodyKeys []string
}

type Case[C any] struct {
	Call     func(context.Context, C) error
	Status   int
	Response string
	Want     Request
}

func Run[C, O any](t *testing.T, newClient func(server string, opts ...O) (C, error), record func(*testing.T, *http.Request) Request, cases map[string]Case[C]) {
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var got Request
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = record(t, r)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.Status)
				_, _ = w.Write([]byte(tc.Response))
			}))
			defer srv.Close()
			c, err := newClient(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.Call(t.Context(), c); err != nil {
				t.Fatal(err)
			}
			AssertRequest(t, got, tc.Want)
		})
	}
}

func RecordObject(t *testing.T, r *http.Request) Request {
	got, raw := RecordRaw(t, r)
	if len(raw) == 0 {
		return got
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	got.BodyKeys = slices.Sorted(maps.Keys(body))
	return got
}

func RecordRaw(t *testing.T, r *http.Request) (Request, []byte) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return Request{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query()}, raw
}

func AssertRequest(t *testing.T, got, want Request) {
	t.Helper()
	if got.Method != want.Method || got.Path != want.Path || got.Query.Encode() != want.Query.Encode() {
		t.Errorf("request = %s %s?%s, want %s %s?%s", got.Method, got.Path, got.Query.Encode(), want.Method, want.Path, want.Query.Encode())
	}
	if !slices.Equal(got.BodyKeys, want.BodyKeys) {
		t.Errorf("body keys = %v, want %v", got.BodyKeys, want.BodyKeys)
	}
}

func AssertGeneratedHeader(t *testing.T) {
	boilerplate, err := os.ReadFile("../../../hack/boilerplate.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile("client.gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(generated), string(boilerplate)) {
		t.Errorf("client.gen.go does not start with hack/boilerplate.go.txt")
	}
}
