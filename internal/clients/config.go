// SPDX-FileCopyrightText: 2026 The provider-checkmarx authors
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Cliftonz/provider-checkmarx/apis/v1beta1"
)

const (
	errGetProviderConfig  = "cannot get referenced ProviderConfig"
	errTrackUsage         = "cannot track ProviderConfig usage"
	errExtractCredentials = "cannot extract credentials"
)

var (
	ErrMalformedCredentials = errors.New("Checkmarx One credentials are malformed JSON")
	ErrMissingBaseURL       = errors.New("Checkmarx One credentials are missing baseUrl")
	ErrMissingIAMURL        = errors.New("Checkmarx One credentials are missing iamUrl")
	ErrMissingTenant        = errors.New("Checkmarx One credentials are missing tenant")
	ErrMissingCredentials   = errors.New("Checkmarx One credentials need apiKey or clientId and clientSecret")
	ErrAmbiguousCredentials = errors.New("Checkmarx One credentials must not set apiKey together with clientId or clientSecret")
	ErrMalformedAPIKey      = errors.New("Checkmarx One apiKey is not a well-formed JWT")
)

type Connection struct {
	HTTPClient *http.Client
	BaseURL    string
	IAMURL     string
	Tenant     string
}

type credentials struct {
	BaseURL      string `json:"baseUrl"`
	IAMURL       string `json:"iamUrl"`
	Tenant       string `json:"tenant"`
	APIKey       string `json:"apiKey"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

func NewClient(ctx context.Context, c client.Client, mg resource.ModernManaged) (*Connection, error) {
	configRef := mg.GetProviderConfigReference()
	var pcSpec v1beta1.ProviderConfigSpec
	switch configRef.Kind {
	case v1beta1.ProviderConfigKind:
		pc := &v1beta1.ProviderConfig{}
		if err := c.Get(ctx, types.NamespacedName{Name: configRef.Name, Namespace: mg.GetNamespace()}, pc); err != nil {
			return nil, errors.Wrap(err, errGetProviderConfig)
		}
		pcSpec = pc.Spec
		pcSpec.Credentials.SecretRef.Namespace = mg.GetNamespace()
	case v1beta1.ClusterProviderConfigKind:
		pc := &v1beta1.ClusterProviderConfig{}
		if err := c.Get(ctx, types.NamespacedName{Name: configRef.Name}, pc); err != nil {
			return nil, errors.Wrap(err, errGetProviderConfig)
		}
		pcSpec = pc.Spec
	default:
		panic("providerConfigRef kind must be ProviderConfig or ClusterProviderConfig, got " + configRef.Kind)
	}
	data, err := resource.CommonCredentialExtractor(ctx, pcSpec.Credentials.Source, c, pcSpec.Credentials.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errExtractCredentials)
	}
	var creds credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformedCredentials, err)
	}
	creds.BaseURL = strings.TrimRight(creds.BaseURL, "/")
	creds.IAMURL = strings.TrimRight(creds.IAMURL, "/")
	switch {
	case creds.BaseURL == "":
		return nil, ErrMissingBaseURL
	case creds.IAMURL == "":
		return nil, ErrMissingIAMURL
	case creds.Tenant == "":
		return nil, ErrMissingTenant
	case creds.APIKey == "" && (creds.ClientID == "" || creds.ClientSecret == ""):
		return nil, ErrMissingCredentials
	case creds.APIKey != "" && (creds.ClientID != "" || creds.ClientSecret != ""):
		return nil, ErrAmbiguousCredentials
	}
	transport, err := newBearerTransport(creds)
	if err != nil {
		return nil, err
	}
	if err := resource.NewProviderConfigUsageTracker(c, &v1beta1.ProviderConfigUsage{}).Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}
	return &Connection{
		HTTPClient: &http.Client{Transport: transport},
		BaseURL:    creds.BaseURL,
		IAMURL:     creds.IAMURL,
		Tenant:     creds.Tenant,
	}, nil
}
