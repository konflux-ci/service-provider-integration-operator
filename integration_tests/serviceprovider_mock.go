//
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

package integrationtests

import (
	"context"

	api "github.com/mshaposhnik/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestServiceProvider is an implementation of the serviceprovider.ServiceProvider interface that can be modified by
// supplying custom implementations of each of the interface methods. It provides dummy implementations of them, too, so
// that no null pointer dereferences should occur under normal operation.
type TestServiceProvider struct {
	LookupTokenImpl       func(context.Context, client.Client, *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error)
	PersistMetadataImpl   func(context.Context, client.Client, *api.SPIAccessToken) error
	GetBaseUrlImpl        func() string
	TranslateToScopesImpl func(permission api.Permission) []string
	GetTypeImpl           func() api.ServiceProviderType
	GetOauthEndpointImpl  func() string
}

func (t TestServiceProvider) LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	if t.LookupTokenImpl == nil {
		return nil, nil
	}
	return t.LookupTokenImpl(ctx, cl, binding)
}

func (t TestServiceProvider) PersistMetadata(ctx context.Context, cl client.Client, token *api.SPIAccessToken) error {
	if t.PersistMetadataImpl == nil {
		return nil
	}

	return t.PersistMetadataImpl(ctx, cl, token)
}

func (t TestServiceProvider) GetBaseUrl() string {
	if t.GetBaseUrlImpl == nil {
		return "test-provider://"
	}
	return t.GetBaseUrlImpl()
}

func (t TestServiceProvider) TranslateToScopes(permission api.Permission) []string {
	if t.TranslateToScopesImpl == nil {
		return []string{}
	}
	return t.TranslateToScopesImpl(permission)
}

func (t TestServiceProvider) GetType() api.ServiceProviderType {
	if t.GetTypeImpl == nil {
		return "TestServiceProvider"
	}
	return t.GetTypeImpl()
}

func (t TestServiceProvider) GetOAuthEndpoint() string {
	if t.GetOauthEndpointImpl == nil {
		return ""
	}
	return t.GetOauthEndpointImpl()
}

func (t *TestServiceProvider) Reset() {
	t.LookupTokenImpl = nil
	t.GetBaseUrlImpl = nil
	t.TranslateToScopesImpl = nil
	t.GetTypeImpl = nil
	t.GetOauthEndpointImpl = nil
	t.PersistMetadataImpl = nil
}

// LookupConcreteToken returns a function that can be used as the TestServiceProvider.LookupTokenImpl that just returns
// a freshly loaded version of the provided token. The token is a pointer to a pointer to the token so that this can
// also support lazily initialized tokens.
func LookupConcreteToken(tokenPointer **api.SPIAccessToken) func(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	return func(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
		if *tokenPointer == nil {
			return nil, nil
		}

		freshToken := &api.SPIAccessToken{}
		if err := cl.Get(ctx, client.ObjectKeyFromObject(*tokenPointer), freshToken); err != nil {
			return nil, err
		}
		return freshToken, nil
	}
}

// PersistConcreteMetadata returns a function that can be used as the TestServiceProvider.PersistMetadataImpl that
// stores the provided metadata to any token.
func PersistConcreteMetadata(metadata *api.TokenMetadata) func(context.Context, client.Client, *api.SPIAccessToken) error {
	return func(ctx context.Context, cl client.Client, token *api.SPIAccessToken) error {
		token.Status.TokenMetadata = metadata
		return cl.Status().Update(ctx, token)
	}
}
