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
	"github.com/go-jose/go-jose/v3/jwt"
	"k8s.io/apiserver/pkg/authentication/user"
)

// AuthenticatedOAuthState is the state passed by the OAuth service to the service provider. In addition to the data
// defined by the AnonymousOAuthState, it also includes the kubernetes identity of the caller as well as the actual
// authorization header provided by the caller. This then can be used after completing the OAuth flow to perform
// requests to the Kubernetes API in the name of the original caller.
type AuthenticatedOAuthState struct {
	AnonymousOAuthState
	KubernetesIdentity  user.DefaultInfo `json:"kubernetesIdentity"`
	AuthorizationHeader string           `json:"authorizationHeader"`
}

// EncodeAuthenticated encodes the `AuthenticatedOAuthState` as a string to be used as a state query parameter in the
// OAuth URL.
func (s *Codec) EncodeAuthenticated(state *AuthenticatedOAuthState) (string, error) {
	return jwt.Signed(s.signer).Claims(state).CompactSerialize()
}

// ParseAuthenticated parses the string from the query parameter to an `AuthenticatedOAuthState` struct.
func (s *Codec) ParseAuthenticated(state string) (AuthenticatedOAuthState, error) {
	token, err := jwt.ParseSigned(state)
	if err != nil {
		return AuthenticatedOAuthState{}, err
	}

	parsedState := AuthenticatedOAuthState{}
	if err := token.Claims(s.signingSecret, &parsedState); err != nil {
		return AuthenticatedOAuthState{}, err
	}

	if err := validateAnonymousState(&parsedState.AnonymousOAuthState); err != nil {
		return AuthenticatedOAuthState{}, err
	}

	return parsedState, nil
}
