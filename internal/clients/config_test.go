// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/Cliftonz/provider-checkmarx/apis/v1beta1"
)

const namespace = "team-a"

func credentialsSpec(secretNamespace string) v1beta1.ProviderConfigSpec {
	return v1beta1.ProviderConfigSpec{Credentials: v1beta1.ProviderCredentials{
		Source: xpv2.CredentialsSourceSecret,
		CommonCredentialSelectors: xpv2.CommonCredentialSelectors{SecretRef: &xpv2.SecretKeySelector{
			SecretReference: xpv2.SecretReference{Name: "checkmarx-creds", Namespace: secretNamespace},
			Key:             "credentials",
		}},
	}}
}

func kubeClient(t *testing.T, pc client.Object, secretNamespace, credentials string) client.WithWatch {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := v1beta1.SchemeBuilder.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "checkmarx-creds", Namespace: secretNamespace},
		Data:       map[string][]byte{"credentials": []byte(credentials)},
	}
	return kfake.NewClientBuilder().WithScheme(s).WithObjects(pc, secret).Build()
}

func managed(kind string) *fake.ModernManaged {
	mg := &fake.ModernManaged{ObjectMeta: metav1.ObjectMeta{Name: "my-project", Namespace: namespace, UID: "mg-uid"}}
	mg.SetProviderConfigReference(&xpv2.ProviderConfigReference{Kind: kind, Name: "default"})
	return mg
}

func namespacedProviderConfig() *v1beta1.ProviderConfig {
	return &v1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: namespace},
		Spec:       credentialsSpec(""),
	}
}

func newClientError(t *testing.T, credentials string) error {
	t.Helper()
	kube := kubeClient(t, namespacedProviderConfig(), namespace, credentials)
	_, err := NewClient(context.Background(), kube, managed(v1beta1.ProviderConfigKind))
	return err
}

func TestNewClientRejectsMalformedJSON(t *testing.T) {
	err := newClientError(t, `{"baseUrl":`)
	var syntaxErr *json.SyntaxError
	if !errors.Is(err, ErrMalformedCredentials) || !errors.As(err, &syntaxErr) {
		t.Fatalf("NewClient() error = %v, want %v wrapping a JSON syntax error", err, ErrMalformedCredentials)
	}
}

func TestNewClientRequiresBaseURL(t *testing.T) {
	err := newClientError(t, `{"iamUrl":"https://iam.checkmarx.net","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`)
	if !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("NewClient() error = %v, want %v", err, ErrMissingBaseURL)
	}
}

func TestNewClientRequiresIAMURL(t *testing.T) {
	err := newClientError(t, `{"baseUrl":"https://ast.checkmarx.net","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`)
	if !errors.Is(err, ErrMissingIAMURL) {
		t.Fatalf("NewClient() error = %v, want %v", err, ErrMissingIAMURL)
	}
}

func TestNewClientRequiresTenant(t *testing.T) {
	err := newClientError(t, `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`)
	if !errors.Is(err, ErrMissingTenant) {
		t.Fatalf("NewClient() error = %v, want %v", err, ErrMissingTenant)
	}
}

func TestNewClientRequiresCredentials(t *testing.T) {
	cases := map[string]string{
		"Neither":          `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme"}`,
		"ClientIDOnly":     `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme","clientId":"id"}`,
		"ClientSecretOnly": `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme","clientSecret":"s"}`,
	}
	for name, credentials := range cases {
		t.Run(name, func(t *testing.T) {
			err := newClientError(t, credentials)
			if !errors.Is(err, ErrMissingCredentials) {
				t.Fatalf("NewClient() error = %v, want %v", err, ErrMissingCredentials)
			}
		})
	}
}

func TestNewClientRejectsBothCredentialKinds(t *testing.T) {
	cases := map[string]string{
		"APIKeyAndClientID":     `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s","clientId":"id"`,
		"APIKeyAndClientSecret": `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s","clientSecret":"s"`,
		"APIKeyAndBoth":         `"apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s","clientId":"id","clientSecret":"s"`,
	}
	for name, fields := range cases {
		t.Run(name, func(t *testing.T) {
			err := newClientError(t, `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme",`+fields+`}`)
			if !errors.Is(err, ErrAmbiguousCredentials) {
				t.Fatalf("NewClient() error = %v, want %v", err, ErrAmbiguousCredentials)
			}
		})
	}
}

func TestNewClientPanicsOnUnknownProviderConfigKind(t *testing.T) {
	kube := kubeClient(t, namespacedProviderConfig(), namespace, `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`)
	want := "providerConfigRef kind must be ProviderConfig or ClusterProviderConfig, got UnregisteredKind"
	defer func() {
		if got := recover(); got != want {
			t.Fatalf("NewClient() panic = %v, want %q", got, want)
		}
	}()
	_, _ = NewClient(context.Background(), kube, managed("UnregisteredKind"))
}

func TestNewClientReturnsConnectionDetails(t *testing.T) {
	kube := kubeClient(t, namespacedProviderConfig(), namespace, `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`)
	conn, err := NewClient(context.Background(), kube, managed(v1beta1.ProviderConfigKind))
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}
	if conn.BaseURL != "https://ast.checkmarx.net" || conn.IAMURL != "https://iam.checkmarx.net" || conn.Tenant != "acme" {
		t.Fatalf("Connection = %+v, want baseUrl, iamUrl and tenant from the Secret", conn)
	}
}

func TestNewClientTrimsTrailingSlashes(t *testing.T) {
	kube := kubeClient(t, namespacedProviderConfig(), namespace, `{"baseUrl":"https://ast.checkmarx.net/","iamUrl":"https://iam.checkmarx.net/","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`)
	conn, err := NewClient(context.Background(), kube, managed(v1beta1.ProviderConfigKind))
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}
	if conn.BaseURL != "https://ast.checkmarx.net" || conn.IAMURL != "https://iam.checkmarx.net" {
		t.Fatalf("Connection = %+v, want URLs without trailing slashes", conn)
	}
}

func TestNewClientNamespacedConfigUsesResourceNamespace(t *testing.T) {
	pc := &v1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: namespace},
		Spec:       credentialsSpec("elsewhere"),
	}
	kube := kubeClient(t, pc, namespace, `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`)
	if _, err := NewClient(context.Background(), kube, managed(v1beta1.ProviderConfigKind)); err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}
}

func TestNewClientClusterConfigUsesSecretRefNamespace(t *testing.T) {
	pc := &v1beta1.ClusterProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec:       credentialsSpec("crossplane-system"),
	}
	kube := kubeClient(t, pc, "crossplane-system", `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`)
	if _, err := NewClient(context.Background(), kube, managed(v1beta1.ClusterProviderConfigKind)); err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}
}

func TestNewClientTracksProviderConfigUsage(t *testing.T) {
	kube := kubeClient(t, namespacedProviderConfig(), namespace, `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`)
	if _, err := NewClient(context.Background(), kube, managed(v1beta1.ProviderConfigKind)); err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}
	usages := &v1beta1.ProviderConfigUsageList{}
	if err := kube.List(context.Background(), usages); err != nil {
		t.Fatal(err)
	}
	if len(usages.Items) != 1 || usages.Items[0].ProviderConfigReference.Name != "default" {
		t.Fatalf("ProviderConfigUsages = %+v, want one usage of ProviderConfig default", usages.Items)
	}
}

func TestNewClientRejectsMalformedAPIKey(t *testing.T) {
	cases := map[string]string{
		"NotThreeParts":    "opaque-key",
		"PayloadNotBase64": "h.!!!.s",
		"PayloadNotJSON":   "h.bm90IGpzb24.s",
		"EmptyAzp":         "h.eyJhenAiOiIifQ.s",
	}
	for name, apiKey := range cases {
		t.Run(name, func(t *testing.T) {
			err := newClientError(t, `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme","apiKey":"`+apiKey+`"}`)
			if !errors.Is(err, ErrMalformedAPIKey) {
				t.Fatalf("NewClient() error = %v, want %v", err, ErrMalformedAPIKey)
			}
		})
	}
}

func TestNewClientRejectsSlashOnlyURLs(t *testing.T) {
	cases := map[string]struct {
		credentials string
		want        error
	}{
		"BaseURL": {credentials: `{"baseUrl":"/","iamUrl":"https://iam.checkmarx.net","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`, want: ErrMissingBaseURL},
		"IAMURL":  {credentials: `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"/","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`, want: ErrMissingIAMURL},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := newClientError(t, tc.credentials); !errors.Is(err, tc.want) {
				t.Fatalf("NewClient() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewClientDoesNotTrackUsageForInvalidCredentials(t *testing.T) {
	kube := kubeClient(t, namespacedProviderConfig(), namespace, `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`)
	if _, err := NewClient(context.Background(), kube, managed(v1beta1.ProviderConfigKind)); !errors.Is(err, ErrMissingTenant) {
		t.Fatalf("NewClient() error = %v, want %v", err, ErrMissingTenant)
	}
	usages := &v1beta1.ProviderConfigUsageList{}
	if err := kube.List(context.Background(), usages); err != nil {
		t.Fatal(err)
	}
	if len(usages.Items) != 0 {
		t.Fatalf("ProviderConfigUsages = %d, want 0", len(usages.Items))
	}
}

func TestNewClientReportsMissingProviderConfig(t *testing.T) {
	cases := map[string]struct {
		pc   client.Object
		kind string
	}{
		"ProviderConfig":        {pc: &v1beta1.ProviderConfig{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: namespace}, Spec: credentialsSpec("")}, kind: v1beta1.ProviderConfigKind},
		"ClusterProviderConfig": {pc: &v1beta1.ClusterProviderConfig{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Spec: credentialsSpec(namespace)}, kind: v1beta1.ClusterProviderConfigKind},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kube := kubeClient(t, tc.pc, namespace, `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`)
			_, err := NewClient(context.Background(), kube, managed(tc.kind))
			if !kerrors.IsNotFound(err) || !strings.HasPrefix(err.Error(), errGetProviderConfig) {
				t.Fatalf("NewClient() error = %v, want a not-found error wrapped with %q", err, errGetProviderConfig)
			}
		})
	}
}

func TestNewClientReportsMissingSecret(t *testing.T) {
	kube := kubeClient(t, namespacedProviderConfig(), "elsewhere", `{}`)
	_, err := NewClient(context.Background(), kube, managed(v1beta1.ProviderConfigKind))
	if !kerrors.IsNotFound(err) || !strings.HasPrefix(err.Error(), errExtractCredentials) {
		t.Fatalf("NewClient() error = %v, want a not-found error wrapped with %q", err, errExtractCredentials)
	}
}

func TestNewClientReportsUsageTrackingFailure(t *testing.T) {
	rejected := errors.New("usage create rejected")
	kube := interceptor.NewClient(kubeClient(t, namespacedProviderConfig(), namespace, `{"baseUrl":"https://ast.checkmarx.net","iamUrl":"https://iam.checkmarx.net","tenant":"acme","apiKey":"h.eyJzdWIiOiJ1c2VyIn0.s"}`), interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*v1beta1.ProviderConfigUsage); ok {
				return rejected
			}
			return c.Create(ctx, obj, opts...)
		},
	})

	_, err := NewClient(context.Background(), kube, managed(v1beta1.ProviderConfigKind))
	if !errors.Is(err, rejected) || !strings.HasPrefix(err.Error(), errTrackUsage) {
		t.Fatalf("NewClient() error = %v, want %v wrapped with %q", err, rejected, errTrackUsage)
	}
}
