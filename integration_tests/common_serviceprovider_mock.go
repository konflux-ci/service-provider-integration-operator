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

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CommonServiceProvider struct {
	LookupTokenImpl           func(context.Context, client.Client, *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error)
	PersistMetadataImpl       func(context.Context, client.Client, *api.SPIAccessToken) error
	GetBaseUrlImpl            func() string
	TranslateToScopesImpl     func(permission api.Permission) []string
	GetTypeImpl               func() api.ServiceProviderType
	GetOauthEndpointImpl      func() string
	CheckRepositoryAccessImpl func(context.Context, client.Client, *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error)
	MapTokenImpl              func(context.Context, *api.SPIAccessTokenBinding, *api.SPIAccessToken, *api.Token) (serviceprovider.AccessTokenMapper, error)
	ValidateImpl              func(context.Context, serviceprovider.Validated) (serviceprovider.ValidationResult, error)
}

func (t CommonServiceProvider) CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error) {
	if t.CheckRepositoryAccessImpl == nil {
		return &api.SPIAccessCheckStatus{}, nil
	}
	return t.CheckRepositoryAccessImpl(ctx, cl, accessCheck)
}

func (t CommonServiceProvider) LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	if t.LookupTokenImpl == nil {
		return nil, nil
	}
	return t.LookupTokenImpl(ctx, cl, binding)
}

func (t CommonServiceProvider) PersistMetadata(ctx context.Context, cl client.Client, token *api.SPIAccessToken) error {
	if t.PersistMetadataImpl == nil {
		return nil
	}

	return t.PersistMetadataImpl(ctx, cl, token)
}

func (t CommonServiceProvider) GetBaseUrl() string {
	if t.GetBaseUrlImpl == nil {
		return ""
	}
	return t.GetBaseUrlImpl()
}

func (t CommonServiceProvider) TranslateToScopes(permission api.Permission) []string {
	if t.TranslateToScopesImpl == nil {
		return []string{}
	}
	return t.TranslateToScopesImpl(permission)
}

func (t CommonServiceProvider) GetType() api.ServiceProviderType {
	if t.GetTypeImpl == nil {
		return "CommonServiceProvider"
	}
	return t.GetTypeImpl()
}

func (t CommonServiceProvider) GetOAuthEndpoint() string {
	if t.GetOauthEndpointImpl == nil {
		return ""
	}
	return t.GetOauthEndpointImpl()
}

func (t CommonServiceProvider) MapToken(ctx context.Context, binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, tokenData *api.Token) (serviceprovider.AccessTokenMapper, error) {
	if t.MapTokenImpl == nil {
		return serviceprovider.AccessTokenMapper{}, nil
	}

	return t.MapTokenImpl(ctx, binding, token, tokenData)
}

func (t CommonServiceProvider) Validate(ctx context.Context, validated serviceprovider.Validated) (serviceprovider.ValidationResult, error) {
	if t.ValidateImpl == nil {
		return serviceprovider.ValidationResult{}, nil
	}

	return t.ValidateImpl(ctx, validated)
}

func (t *CommonServiceProvider) Reset() {
	t.LookupTokenImpl = nil
	t.GetBaseUrlImpl = nil
	t.TranslateToScopesImpl = nil
	t.GetTypeImpl = nil
	t.GetOauthEndpointImpl = nil
	t.PersistMetadataImpl = nil
	t.CheckRepositoryAccessImpl = nil
	t.MapTokenImpl = nil
	t.ValidateImpl = nil
}
