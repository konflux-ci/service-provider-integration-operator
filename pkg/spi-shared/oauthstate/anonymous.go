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
)

// OAuthInfo is the state that is initially put to the OAuth URL by the operator.
// This state is put by the operator to the status of the SPIAccessToken and points to an endpoint in the OAuth service.
// OAuth service requires kubernetes authentication on this endpoint, reads this data and redirects the caller once
// again to the actual service provider with a random key to its session storage as the OAuth state.
type OAuthInfo struct {
	// TokenName is the name of the SPIAccessToken object for which we are initiating the OAuth flow
	TokenName string `json:"tokenName"`

	// TokenNamespace is the namespace of the SPIAccessToken object for which we are initiating the OAuth flow
	TokenNamespace string `json:"tokenNamespace"`

	// TokenKcpWorkspace is the KCP workspace where SPIAccessToken lives. It's empty in non-KCP environment
	TokenKcpWorkspace string `json:"tokenKcpWorkspace"`

	// Scopes is the list of the service-provider-specific scopes that we require in the service provider
	Scopes []string `json:"scopes"`

	// ServiceProviderType is the type of the service provider
	ServiceProviderType config.ServiceProviderType `json:"serviceProviderType"`

	// ServiceProviderUrl the URL where the service provider is to be reached
	ServiceProviderUrl string `json:"serviceProviderUrl"`
}

// ParseOAuthInfo parses the state from the URL query parameter and returns the anonymous state struct. It is just
// a typed variant of the ParseInto function.
func ParseOAuthInfo(state string) (OAuthInfo, error) {
	parsedState := OAuthInfo{}
	err := ParseInto(state, &parsedState)
	if err != nil {
		return parsedState, fmt.Errorf("error parsing OAuth state %w", err)
	}

	return parsedState, nil
}
