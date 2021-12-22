// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oauthstate

import (
	"fmt"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	"github.com/go-jose/go-jose/v3/jwt"
)

// AnonymousOAuthState is the state that is initially put to the OAuth URL by the operator. It does not hold
// the information about the user that initiated the OAuth flow because the operator most probably doesn't know
// the true identity of the initiating human.
// This state is put by the operator to the status of the SPIAccessToken and points to an endpoint in the OAuth service.
// OAuth service requires kubernetes authentication on this endpoint, enriches the state with identity of the user
// accessing the endpoint and redirects the caller once again to the actual service provider with the state that also
// contains the identity of the requesting caller.
type AnonymousOAuthState struct {
	// TokenName is the name of the SPIAccessToken object for which we are initiating the OAuth flow
	TokenName string `json:"tokenName"`

	// TokenNamespace is the namespace of the SPIAccessToken object for which we are initiating the OAuth flow
	TokenNamespace string `json:"tokenNamespace"`

	// IssuedAt is the timestamp when the state was generated.
	IssuedAt int64 `json:"issuedAt,omitempty"`

	// Scopes is the list of the service-provider-specific scopes that we require in the service provider
	Scopes []string `json:"scopes"`

	// ServiceProviderType is the type of the service provider
	ServiceProviderType config.ServiceProviderType `json:"serviceProviderType"`

	// ServiceProviderUrl the URL where the service provider is to be reached
	ServiceProviderUrl string `json:"serviceProviderUrl"`
}

// EncodeAnonymous encodes the anonymous state as a string to be passed as a query parameter to the OAuth flow initiating
// URL.
func (s *Codec) EncodeAnonymous(state *AnonymousOAuthState) (string, error) {
	return jwt.Signed(s.signer).Claims(state).CompactSerialize()
}

// ParseAnonymous parses the state from the URL query parameter and returns the anonymous state struct.
func (s *Codec) ParseAnonymous(state string) (AnonymousOAuthState, error) {
	token, err := jwt.ParseSigned(state)
	if err != nil {
		return AnonymousOAuthState{}, err
	}

	parsedState := AnonymousOAuthState{}
	if err := token.Claims(s.signingSecret, &parsedState); err != nil {
		return AnonymousOAuthState{}, err
	}

	if err := validateAnonymousState(&parsedState); err != nil {
		return AnonymousOAuthState{}, err
	}

	return parsedState, nil
}

func validateAnonymousState(state *AnonymousOAuthState) error {
	if time.Now().Unix() < state.IssuedAt {
		return fmt.Errorf("request from the future")
	}
	return nil
}
