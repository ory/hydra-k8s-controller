// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatusCode string

const (
	StatusRegistrationFailed  StatusCode = "CLIENT_REGISTRATION_FAILED"
	StatusCreateSecretFailed  StatusCode = "SECRET_CREATION_FAILED"
	StatusUpdateFailed        StatusCode = "CLIENT_UPDATE_FAILED"
	StatusInvalidSecret       StatusCode = "INVALID_SECRET"
	StatusInvalidHydraAddress StatusCode = "INVALID_HYDRA_ADDRESS"
)

// HydraAdmin defines the desired hydra admin instance to use for OAuth2Client
type HydraAdmin struct {
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern=`(^$|^https?://.*)`
	//
	// URL is the URL for the hydra instance on
	// which to set up the client. This value will override the value
	// provided to `--hydra-url`
	URL string `json:"url,omitempty"`

	// +kubebuilder:validation:Maximum=65535
	//
	// Port is the port for the hydra instance on
	// which to set up the client. This value will override the value
	// provided to `--hydra-port`
	Port int `json:"port,omitempty"`

	// +kubebuilder:validation:Pattern=(^$|^/.*)
	//
	// Endpoint is the endpoint for the hydra instance on which
	// to set up the client. This value will override the value
	// provided to `--endpoint` (defaults to `"/clients"` in the
	// application)
	Endpoint string `json:"endpoint,omitempty"`

	// +kubebuilder:validation:Pattern=(^$|https?|off)
	//
	// ForwardedProto overrides the `--forwarded-proto` flag. The
	// value "off" will force this to be off even if
	// `--forwarded-proto` is specified
	ForwardedProto string `json:"forwardedProto,omitempty"`

	// ApiKeySecretRef is an object to define the secret which contains
	// Ory Network API Key
	ApiKeySecretRef ApiKeySecretRef `json:"apiKeySecretRef,omitempty"`
}

// ApiKeySecretRef contains Secret details for the API Key
type ApiKeySecretRef struct {
	// Name of the secret containing the API Key
	Name string `json:"name,omitempty"`
	// Key of the secret for the API key
	Key string `json:"key,omitempty"`
	// Namespace of the secret if different from hydra-maester controller
	Namespace string `json:"namespace,omitempty"`
}

// TokenLifespans defines the desired token durations by grant type for OAuth2Client
type TokenLifespans struct {
	// +kubebuilder:validation:Pattern=[0-9]+(ns|us|ms|s|m|h)
	//
	// AuthorizationCodeGrantAccessTokenLifespan is the access token lifespan
	// issued on an authorization_code grant.
	AuthorizationCodeGrantAccessTokenLifespan string `json:"authorization_code_grant_access_token_lifespan,omitempty"`

	// +kubebuilder:validation:Pattern=[0-9]+(ns|us|ms|s|m|h)
	//
	// AuthorizationCodeGrantIdTokenLifespan is the id token lifespan
	// issued on an authorization_code grant.
	AuthorizationCodeGrantIdTokenLifespan string `json:"authorization_code_grant_id_token_lifespan,omitempty"`

	// +kubebuilder:validation:Pattern=[0-9]+(ns|us|ms|s|m|h)
	//
	// AuthorizationCodeGrantRefreshTokenLifespan is the refresh token lifespan
	// issued on an authorization_code grant.
	AuthorizationCodeGrantRefreshTokenLifespan string `json:"authorization_code_grant_refresh_token_lifespan,omitempty"`

	// +kubebuilder:validation:Pattern=[0-9]+(ns|us|ms|s|m|h)
	//
	// AuthorizationCodeGrantRefreshTokenLifespan is the access token lifespan
	// issued on a client_credentials grant.
	ClientCredentialsGrantAccessTokenLifespan string `json:"client_credentials_grant_access_token_lifespan,omitempty"`

	// +kubebuilder:validation:Pattern=[0-9]+(ns|us|ms|s|m|h)
	//
	// ImplicitGrantAccessTokenLifespan is the access token lifespan
	// issued on an implicit grant.
	ImplicitGrantAccessTokenLifespan string `json:"implicit_grant_access_token_lifespan,omitempty"`

	// +kubebuilder:validation:Pattern=[0-9]+(ns|us|ms|s|m|h)
	//
	// ImplicitGrantIdTokenLifespan is the id token lifespan
	// issued on an implicit grant.
	ImplicitGrantIdTokenLifespan string `json:"implicit_grant_id_token_lifespan,omitempty"`

	// +kubebuilder:validation:Pattern=[0-9]+(ns|us|ms|s|m|h)
	//
	// JwtBearerGrantAccessTokenLifespan is the access token lifespan
	// issued on a jwt_bearer grant.
	JwtBearerGrantAccessTokenLifespan string `json:"jwt_bearer_grant_access_token_lifespan,omitempty"`

	// +kubebuilder:validation:Pattern=[0-9]+(ns|us|ms|s|m|h)
	//
	// RefreshTokenGrantAccessTokenLifespan is the access token lifespan
	// issued on a refresh_token grant.
	RefreshTokenGrantAccessTokenLifespan string `json:"refresh_token_grant_access_token_lifespan,omitempty"`

	// +kubebuilder:validation:Pattern=[0-9]+(ns|us|ms|s|m|h)
	//
	// RefreshTokenGrantIdTokenLifespan is the id token lifespan
	// issued on a refresh_token grant.
	RefreshTokenGrantIdTokenLifespan string `json:"refresh_token_grant_id_token_lifespan,omitempty"`

	// +kubebuilder:validation:Pattern=[0-9]+(ns|us|ms|s|m|h)
	//
	// RefreshTokenGrantRefreshTokenLifespan is the refresh token lifespan
	// issued on a refresh_token grant.
	RefreshTokenGrantRefreshTokenLifespan string `json:"refresh_token_grant_refresh_token_lifespan,omitempty"`
}

// OAuth2ClientSpec defines the desired state of OAuth2Client
type OAuth2ClientSpec struct {

	// ClientName is the human-readable string name of the client to be presented to the end-user during authorization.
	ClientName string `json:"clientName,omitempty"`

	// +kubebuilder:validation:MaxItems=4
	// +kubebuilder:validation:MinItems=1
	//
	// GrantTypes is an array of grant types the client is allowed to use.
	GrantTypes []GrantType `json:"grantTypes"`

	// +kubebuilder:validation:MaxItems=3
	// +kubebuilder:validation:MinItems=1
	//
	// ResponseTypes is an array of the OAuth 2.0 response type strings that the client can
	// use at the authorization endpoint.
	ResponseTypes []ResponseType `json:"responseTypes,omitempty"`

	// RedirectURIs is an array of the redirect URIs allowed for the application
	RedirectURIs []RedirectURI `json:"redirectUris,omitempty"`

	// PostLogoutRedirectURIs is an array of the post logout redirect URIs allowed for the application
	PostLogoutRedirectURIs []RedirectURI `json:"postLogoutRedirectUris,omitempty"`

	// AllowedCorsOrigins is an array of allowed CORS origins
	AllowedCorsOrigins []RedirectURI `json:"allowedCorsOrigins,omitempty"`

	// Audience is a whitelist defining the audiences this client is allowed to request tokens for
	Audience []string `json:"audience,omitempty"`

	// +kubebuilder:validation:Pattern=([a-zA-Z0-9\.\*]+\s?)*
	// +kubebuilder:deprecatedversion:warning="Property scope is deprecated. Use scopeArray instead."
	//
	// Scope is a string containing a space-separated list of scope values (as
	// described in Section 3.3 of OAuth 2.0 [RFC6749]) that the client
	// can use when requesting access tokens.
	// Use scopeArray instead.
	Scope string `json:"scope,omitempty"`

	// Scope is an array of scope values (as described in Section 3.3 of OAuth 2.0 [RFC6749])
	// that the client can use when requesting access tokens.
	ScopeArray []string `json:"scopeArray,omitempty"`

	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*
	//
	// SecretName points to the K8s secret that contains this client's ID and password
	SecretName string `json:"secretName"`

	// SkipConsent skips the consent screen for this client.
	// +kubebuilder:validation:type=bool
	// +kubebuilder:default=false
	SkipConsent bool `json:"skipConsent,omitempty"`

	// HydraAdmin is the optional configuration to use for managing
	// this client
	HydraAdmin HydraAdmin `json:"hydraAdmin,omitempty"`

	// +kubebuilder:validation:Enum=client_secret_basic;client_secret_post;private_key_jwt;none
	//
	// Indication which authentication method should be used for the token endpoint
	TokenEndpointAuthMethod TokenEndpointAuthMethod `json:"tokenEndpointAuthMethod,omitempty"`

	// TokenLifespans is the configuration to use for managing different token lifespans
	// depending on the used grant type.
	TokenLifespans TokenLifespans `json:"tokenLifespans,omitempty"`

	// +kubebuilder:validation:Type=object
	// +nullable
	// +optional
	//
	// Metadata is arbitrary data
	Metadata apiextensionsv1.JSON `json:"metadata,omitempty"`

	// +kubebuilder:validation:type=string
	// +kubebuilder:validation:Pattern=`(^$|^https?://.*)`
	//
	// JwksUri Define the URL where the JSON Web Key Set should be fetched from when performing the private_key_jwt client authentication method.
	JwksUri string `json:"jwksUri,omitempty"`

	// +kubebuilder:validation:type=bool
	// +kubebuilder:default=false
	//
	// FrontChannelLogoutSessionRequired Boolean value specifying whether the RP requires that iss (issuer) and sid (session ID) query parameters be included to identify the RP session with the OP when the frontchannel_logout_uri is used
	FrontChannelLogoutSessionRequired bool `json:"frontChannelLogoutSessionRequired,omitempty"`

	// +kubebuilder:validation:type=string
	// +kubebuilder:validation:Pattern=`(^$|^https?://.*)`
	//
	// FrontChannelLogoutURI RP URL that will cause the RP to log itself out when rendered in an iframe by the OP. An iss (issuer) query parameter and a sid (session ID) query parameter MAY be included by the OP to enable the RP to validate the request and to determine which of the potentially multiple sessions is to be logged out; if either is included, both MUST be
	FrontChannelLogoutURI string `json:"frontChannelLogoutURI,omitempty"`

	// +kubebuilder:validation:type=bool
	// +kubebuilder:default=false
	//
	// BackChannelLogoutSessionRequired Boolean value specifying whether the RP requires that a sid (session ID) Claim be included in the Logout Token to identify the RP session with the OP when the backchannel_logout_uri is used. If omitted, the default value is false.
	BackChannelLogoutSessionRequired bool `json:"backChannelLogoutSessionRequired,omitempty"`

	// +kubebuilder:validation:type=string
	// +kubebuilder:validation:Pattern=`(^$|^https?://.*)`
	//
	// BackChannelLogoutURI RP URL that will cause the RP to log itself out when sent a Logout Token by the OP
	BackChannelLogoutURI string `json:"backChannelLogoutURI,omitempty"`

	// +kubebuilder:validation:Enum=1;2
	//
	// Indicates if a deleted OAuth2Client custom resource should delete the database row or not.
	// Value 1 means deletion of the OAuth2 client, value 2 means keep an orphan oauth2 client.
	DeletionPolicy OAuth2ClientDeletionPolicy `json:"deletionPolicy,omitempty"`
}

// GrantType represents an OAuth 2.0 grant type
// +kubebuilder:validation:Enum=client_credentials;authorization_code;implicit;refresh_token
type GrantType string

// ResponseType represents an OAuth 2.0 response type strings
// +kubebuilder:validation:Enum=id_token;code;token;code token;code id_token;id_token token;code id_token token
type ResponseType string

// RedirectURI represents a redirect URI for the client
// +kubebuilder:validation:Pattern=`\w+:/?/?[^\s]+`
type RedirectURI string

// TokenEndpointAuthMethod represents an authentication method for token endpoint
// +kubebuilder:validation:Enum=client_secret_basic;client_secret_post;private_key_jwt;none
type TokenEndpointAuthMethod string

// OAuth2ClientStatus defines the observed state of OAuth2Client
type OAuth2ClientStatus struct {
	// ObservedGeneration represents the most recent generation observed by the daemon set controller.
	ObservedGeneration  int64                   `json:"observedGeneration,omitempty"`
	ReconciliationError ReconciliationError     `json:"reconciliationError,omitempty"`
	Conditions          []OAuth2ClientCondition `json:"conditions,omitempty"`
}

// ReconciliationError represents an error that occurred during the reconciliation process
type ReconciliationError struct {
	// Code is the status code of the reconciliation error
	Code StatusCode `json:"statusCode,omitempty"`
	// Description is the description of the reconciliation error
	Description string `json:"description,omitempty"`
}

// OAuth2ClientCondition contains condition information for an OAuth2Client
type OAuth2ClientCondition struct {
	Type   OAuth2ClientConditionType `json:"type"`
	Status ConditionStatus           `json:"status"`
}

type OAuth2ClientConditionType string

const (
	OAuth2ClientConditionReady = "Ready"
)

// OAuth2ClientDeletionPolicy represents if a deleted oauth2 client object should delete the database row or not.
type OAuth2ClientDeletionPolicy int

const (
	OAuth2ClientDeletionPolicyDelete = iota + 1
	OAuth2ClientDeletionPolicyOrphan
)

// +kubebuilder:validation:Enum=True;False;Unknown
type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OAuth2Client is the Schema for the oauth2clients API
type OAuth2Client struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OAuth2ClientSpec   `json:"spec,omitempty"`
	Status OAuth2ClientStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OAuth2ClientList contains a list of OAuth2Client
type OAuth2ClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OAuth2Client `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OAuth2Client{}, &OAuth2ClientList{})
}
