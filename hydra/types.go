// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package hydra

import (
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"strings"

	"k8s.io/utils/ptr"

	hydrav1alpha1 "github.com/ory/hydra-maester/api/v1alpha1"
)

// OAuth2ClientJSON represents an OAuth2 client digestible by ORY Hydra
type OAuth2ClientJSON struct {
	ClientName                                 string          `json:"client_name,omitempty"`
	ClientID                                   *string         `json:"client_id,omitempty"`
	Secret                                     *string         `json:"client_secret,omitempty"`
	GrantTypes                                 []string        `json:"grant_types"`
	RedirectURIs                               []string        `json:"redirect_uris,omitempty"`
	PostLogoutRedirectURIs                     []string        `json:"post_logout_redirect_uris,omitempty"`
	AllowedCorsOrigins                         []string        `json:"allowed_cors_origins,omitempty"`
	ResponseTypes                              []string        `json:"response_types,omitempty"`
	Audience                                   []string        `json:"audience,omitempty"`
	Scope                                      string          `json:"scope"`
	SkipConsent                                bool            `json:"skip_consent,omitempty"`
	Owner                                      string          `json:"owner"`
	TokenEndpointAuthMethod                    string          `json:"token_endpoint_auth_method,omitempty"`
	Metadata                                   json.RawMessage `json:"metadata,omitempty"`
	JwksUri                                    string          `json:"jwks_uri,omitempty" validate:"required_if=TokenEndpointAuthMethod private_key_jwt"`
	FrontChannelLogoutSessionRequired          bool            `json:"frontchannel_logout_session_required"`
	FrontChannelLogoutURI                      string          `json:"frontchannel_logout_uri"`
	BackChannelLogoutSessionRequired           bool            `json:"backchannel_logout_session_required"`
	BackChannelLogoutURI                       string          `json:"backchannel_logout_uri"`
	AuthorizationCodeGrantAccessTokenLifespan  string          `json:"authorization_code_grant_access_token_lifespan,omitempty"`
	AuthorizationCodeGrantIdTokenLifespan      string          `json:"authorization_code_grant_id_token_lifespan,omitempty"`
	AuthorizationCodeGrantRefreshTokenLifespan string          `json:"authorization_code_grant_refresh_token_lifespan,omitempty"`
	ClientCredentialsGrantAccessTokenLifespan  string          `json:"client_credentials_grant_access_token_lifespan,omitempty"`
	ImplicitGrantAccessTokenLifespan           string          `json:"implicit_grant_access_token_lifespan,omitempty"`
	ImplicitGrantIdTokenLifespan               string          `json:"implicit_grant_id_token_lifespan,omitempty"`
	JwtBearerGrantAccessTokenLifespan          string          `json:"jwt_bearer_grant_access_token_lifespan,omitempty"`
	RefreshTokenGrantAccessTokenLifespan       string          `json:"refresh_token_grant_access_token_lifespan,omitempty"`
	RefreshTokenGrantIdTokenLifespan           string          `json:"refresh_token_grant_id_token_lifespan,omitempty"`
	RefreshTokenGrantRefreshTokenLifespan      string          `json:"refresh_token_grant_refresh_token_lifespan,omitempty"`
}

// Oauth2ClientCredentials represents client ID and password fetched from a
// Kubernetes secret
type Oauth2ClientCredentials struct {
	ID       []byte
	Password []byte
}

func (oj *OAuth2ClientJSON) WithCredentials(credentials *Oauth2ClientCredentials) *OAuth2ClientJSON {
	oj.ClientID = ptr.To(string(credentials.ID))
	if credentials.Password != nil {
		oj.Secret = ptr.To(string(credentials.Password))
	}
	return oj
}

// FromOAuth2Client converts an OAuth2Client into a OAuth2ClientJSON object that represents an OAuth2 InternalClient digestible by ORY Hydra
func FromOAuth2Client(c *hydrav1alpha1.OAuth2Client) (*OAuth2ClientJSON, error) {
	meta, err := json.Marshal(c.Spec.Metadata)
	if err != nil {
		return nil, fmt.Errorf("unable to encode `metadata` property value to json: %w", err)
	}

	if c.Spec.Scope != "" {
		fmt.Println("Property `scope` in client '" + c.Name + "' is deprecated. Rather use scopeArray.")
	}

	var scope = c.Spec.Scope
	if c.Spec.ScopeArray != nil {
		scope = strings.Trim(strings.Join(c.Spec.ScopeArray, " ")+" "+scope, " ")
	}

	client := &OAuth2ClientJSON{
		ClientName:                        c.Spec.ClientName,
		GrantTypes:                        grantToStringSlice(c.Spec.GrantTypes),
		ResponseTypes:                     responseToStringSlice(c.Spec.ResponseTypes),
		RedirectURIs:                      redirectToStringSlice(c.Spec.RedirectURIs),
		PostLogoutRedirectURIs:            redirectToStringSlice(c.Spec.PostLogoutRedirectURIs),
		AllowedCorsOrigins:                redirectToStringSlice(c.Spec.AllowedCorsOrigins),
		Audience:                          c.Spec.Audience,
		Scope:                             scope,
		SkipConsent:                       c.Spec.SkipConsent,
		Owner:                             fmt.Sprintf("%s/%s", c.Name, c.Namespace),
		TokenEndpointAuthMethod:           string(c.Spec.TokenEndpointAuthMethod),
		Metadata:                          meta,
		JwksUri:                           c.Spec.JwksUri,
		FrontChannelLogoutURI:             c.Spec.BackChannelLogoutURI,
		FrontChannelLogoutSessionRequired: c.Spec.BackChannelLogoutSessionRequired,
		BackChannelLogoutSessionRequired:  c.Spec.BackChannelLogoutSessionRequired,
		BackChannelLogoutURI:              c.Spec.BackChannelLogoutURI,
		AuthorizationCodeGrantAccessTokenLifespan:  c.Spec.TokenLifespans.AuthorizationCodeGrantAccessTokenLifespan,
		AuthorizationCodeGrantIdTokenLifespan:      c.Spec.TokenLifespans.AuthorizationCodeGrantIdTokenLifespan,
		AuthorizationCodeGrantRefreshTokenLifespan: c.Spec.TokenLifespans.AuthorizationCodeGrantRefreshTokenLifespan,
		ClientCredentialsGrantAccessTokenLifespan:  c.Spec.TokenLifespans.ClientCredentialsGrantAccessTokenLifespan,
		ImplicitGrantAccessTokenLifespan:           c.Spec.TokenLifespans.ImplicitGrantAccessTokenLifespan,
		ImplicitGrantIdTokenLifespan:               c.Spec.TokenLifespans.ImplicitGrantIdTokenLifespan,
		JwtBearerGrantAccessTokenLifespan:          c.Spec.TokenLifespans.JwtBearerGrantAccessTokenLifespan,
		RefreshTokenGrantAccessTokenLifespan:       c.Spec.TokenLifespans.RefreshTokenGrantAccessTokenLifespan,
		RefreshTokenGrantIdTokenLifespan:           c.Spec.TokenLifespans.RefreshTokenGrantIdTokenLifespan,
		RefreshTokenGrantRefreshTokenLifespan:      c.Spec.TokenLifespans.RefreshTokenGrantRefreshTokenLifespan,
	}

	validate := validator.New()
	if err := validate.Struct(client); err != nil {
		return nil, err
	}

	return client, nil
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
