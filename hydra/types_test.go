// Copyright © 2024 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package hydra_test

import (
	"testing"

	hydrav1alpha1 "github.com/ory/hydra-maester/api/v1alpha1"
	"github.com/ory/hydra-maester/hydra"
	"github.com/stretchr/testify/assert"
)

func TestTypes(t *testing.T) {
	t.Run("Test ScopeArray", func(t *testing.T) {
		c := hydrav1alpha1.OAuth2Client{
			Spec: hydrav1alpha1.OAuth2ClientSpec{
				ScopeArray: []string{"scope1", "scope2"},
			},
		}

		var parsedClient, err = hydra.FromOAuth2Client(&c)
		if err != nil {
			assert.Fail(t, "unexpected error: %s", err)
		}

		assert.Equal(t, parsedClient.Scope, "scope1 scope2")
	})

	t.Run("Test having both Scope and ScopeArray", func(t *testing.T) {
		c := hydrav1alpha1.OAuth2Client{
			Spec: hydrav1alpha1.OAuth2ClientSpec{
				Scope:      "scope3",
				ScopeArray: []string{"scope1", "scope2"},
			},
		}

		var parsedClient, err = hydra.FromOAuth2Client(&c)
		if err != nil {
			assert.Fail(t, "unexpected error: %s", err)
		}

		assert.Equal(t, parsedClient.Scope, "scope1 scope2 scope3")
	})

	t.Run("Test having jwks uri", func(t *testing.T) {
		c := hydrav1alpha1.OAuth2Client{
			Spec: hydrav1alpha1.OAuth2ClientSpec{
				JwksUri: "https://ory.sh/jwks.json",
			},
		}

		var parsedClient, err = hydra.FromOAuth2Client(&c)
		if err != nil {
			assert.Fail(t, "unexpected error: %s", err)
		}

		assert.Equal(t, parsedClient.JwksUri, "https://ory.sh/jwks.json")
	})

	t.Run("Test jwks uri is required when token endpoint auth method is private_key_jwt", func(t *testing.T) {
		c := hydrav1alpha1.OAuth2Client{
			Spec: hydrav1alpha1.OAuth2ClientSpec{
				TokenEndpointAuthMethod: "private_key_jwt",
			},
		}

		var _, err = hydra.FromOAuth2Client(&c)

		assert.ErrorContains(t, err, "JwksUri")
	})
}
