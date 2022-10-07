// Copyright © 2022 Ory Corp

package hydra

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/utils/pointer"

	hydrav1alpha1 "github.com/ory/hydra-maester/api/v1alpha1"
)

// OAuth2ClientJSON represents an OAuth2 client digestible by ORY Hydra
type OAuth2ClientJSON struct {
	ClientName              string          `json:"client_name,omitempty"`
	ClientID                *string         `json:"client_id,omitempty"`
	Secret                  *string         `json:"client_secret,omitempty"`
	GrantTypes              []string        `json:"grant_types"`
	RedirectURIs            []string        `json:"redirect_uris,omitempty"`
	PostLogoutRedirectURIs  []string        `json:"post_logout_redirect_uris,omitempty"`
	AllowedCorsOrigins      []string        `json:"allowed_cors_origins,omitempty"`
	ResponseTypes           []string        `json:"response_types,omitempty"`
	Audience                []string        `json:"audience,omitempty"`
	Scope                   string          `json:"scope"`
	Owner                   string          `json:"owner"`
	TokenEndpointAuthMethod string          `json:"token_endpoint_auth_method,omitempty"`
	Metadata                json.RawMessage `json:"metadata,omitempty"`
}

// Oauth2ClientCredentials represents client ID and password fetched from a
// Kubernetes secret
type Oauth2ClientCredentials struct {
	ID       []byte
	Password []byte
}

func (oj *OAuth2ClientJSON) WithCredentials(credentials *Oauth2ClientCredentials) *OAuth2ClientJSON {
	oj.ClientID = pointer.StringPtr(string(credentials.ID))
	if credentials.Password != nil {
		oj.Secret = pointer.StringPtr(string(credentials.Password))
	}
	return oj
}

// FromOAuth2Client converts an OAuth2Client into a OAuth2ClientJSON object that represents an OAuth2 InternalClient digestible by ORY Hydra
func FromOAuth2Client(c *hydrav1alpha1.OAuth2Client) (*OAuth2ClientJSON, error) {
	meta, err := json.Marshal(c.Spec.Metadata)
	if err != nil {
		return nil, errors.WithMessage(err, "unable to encode `metadata` property value to json")
	}

	return &OAuth2ClientJSON{
		ClientName:              c.Spec.ClientName,
		GrantTypes:              grantToStringSlice(c.Spec.GrantTypes),
		ResponseTypes:           responseToStringSlice(c.Spec.ResponseTypes),
		RedirectURIs:            redirectToStringSlice(c.Spec.RedirectURIs),
		PostLogoutRedirectURIs:  redirectToStringSlice(c.Spec.PostLogoutRedirectURIs),
		AllowedCorsOrigins:      redirectToStringSlice(c.Spec.AllowedCorsOrigins),
		Audience:                c.Spec.Audience,
		Scope:                   c.Spec.Scope,
		Owner:                   fmt.Sprintf("%s/%s", c.Name, c.Namespace),
		TokenEndpointAuthMethod: string(c.Spec.TokenEndpointAuthMethod),
		Metadata:                meta,
	}, nil
}

func responseToStringSlice(rt []hydrav1alpha1.ResponseType) []string {
	var output = make([]string, len(rt))
	for i, elem := range rt {
		output[i] = string(elem)
	}
	return output
}

func grantToStringSlice(gt []hydrav1alpha1.GrantType) []string {
	var output = make([]string, len(gt))
	for i, elem := range gt {
		output[i] = string(elem)
	}
	return output
}

func redirectToStringSlice(ru []hydrav1alpha1.RedirectURI) []string {
	var output = make([]string, len(ru))
	for i, elem := range ru {
		output[i] = string(elem)
	}
	return output
}
