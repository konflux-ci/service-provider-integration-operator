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
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
)

// AnonymousOAuthState is the state that is initially put to the OAuth URL by the operator. It does not hold
// the information about the user that initiated the OAuth flow because the operator most probably doesn't know
// the true identity of the initiating human.
type AnonymousOAuthState struct {
	TokenName           string                     `json:"tokenName"`
	TokenNamespace      string                     `json:"tokenNamespace"`
	IssuedAt            int64                      `json:"issuedAt,omitempty"`
	Scopes              []string                   `json:"scopes"`
	ServiceProviderType config.ServiceProviderType `json:"serviceProviderType"`
	ServiceProviderUrl  string                     `json:"serviceProviderUrl"`
}

func (s *Codec) EncodeAnonymous(state *AnonymousOAuthState) (string, error) {
	return jwt.Signed(s.signer).Claims(state).CompactSerialize()
}

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
