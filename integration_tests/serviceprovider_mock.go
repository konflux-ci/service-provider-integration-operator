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

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestServiceProvider is an implementation of the serviceprovider.ServiceProvider interface that can be modified by
// supplying custom implementations of each of the interface methods. It provides dummy implementations of them, too, so
// that no null pointer dereferences should occur under normal operation.
type TestServiceProvider struct {
	LookupTokenImpl           func(context.Context, client.Client, *api.SPIAccessTokenBinding) ([]api.SPIAccessToken, error)
	PersistMetadataImpl       func(context.Context, client.Client, *api.SPIAccessToken) error
	GetBaseUrlImpl            func() string
	OAuthScopesForImpl        func(permissions *api.Permissions) []string
	GetTypeImpl               func() api.ServiceProviderType
	GetOauthEndpointImpl      func() string
	CheckRepositoryAccessImpl func(context.Context, client.Client, *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error)
	MapTokenImpl              func(context.Context, *api.SPIAccessTokenBinding, *api.SPIAccessToken, *api.Token) (serviceprovider.AccessTokenMapper, error)
	ValidateImpl              func(context.Context, serviceprovider.Validated) (serviceprovider.ValidationResult, error)
	CustomizeReset            func(provider *TestServiceProvider)
}

func (t TestServiceProvider) CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error) {
	lg := log.FromContext(ctx)
	if t.CheckRepositoryAccessImpl == nil {
		lg.V(logs.DebugLevel).Info("empty impl of CheckRepositoryAccess called")
		return &api.SPIAccessCheckStatus{}, nil
	}
	ret, err := t.CheckRepositoryAccessImpl(ctx, cl, accessCheck)
	lg.V(logs.DebugLevel).Info("assigned impl of CheckRepositoryAccess called", "result", ret, "error", err)
	return ret, err
}

func (t TestServiceProvider) LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) ([]api.SPIAccessToken, error) {
	lg := log.FromContext(ctx)
	if t.LookupTokenImpl == nil {
		lg.V(logs.DebugLevel).Info("empty impl of LookupToken called")
		return nil, nil
	}
	ret, err := t.LookupTokenImpl(ctx, cl, binding)
	lg.V(logs.DebugLevel).Info("assigned impl of LookupToken called", "result", ret, "error", err)
	return ret, err
}

func (t TestServiceProvider) PersistMetadata(ctx context.Context, cl client.Client, token *api.SPIAccessToken) error {
	lg := log.FromContext(ctx)
	if t.PersistMetadataImpl == nil {
		lg.V(logs.DebugLevel).Info("empty impl of PersistMetadata called")
		return nil
	}

	err := t.PersistMetadataImpl(ctx, cl, token)
	lg.V(logs.DebugLevel).Info("assigned impl of PersistMetadata called", "error", err)
	return err
}

func (t TestServiceProvider) GetBaseUrl() string {
	if t.GetBaseUrlImpl == nil {
		log.Log.V(logs.DebugLevel).Info("empty impl of GetBaseUrl called")
		return "test-provider://base"
	}
	ret := t.GetBaseUrlImpl()
	log.Log.V(logs.DebugLevel).Info("assigned impl of GetBaseUrl called", "result", ret)
	return ret
}

func (t TestServiceProvider) OAuthScopesFor(permissions *api.Permissions) []string {
	if t.OAuthScopesForImpl == nil {
		log.Log.V(logs.DebugLevel).Info("empty impl of OAuthScopesFor called")
		return []string{}
	}
	ret := t.OAuthScopesForImpl(permissions)
	log.Log.V(logs.DebugLevel).Info("assigned impl of OAuthScopesFor", "result", ret)
	return ret
}

func (t TestServiceProvider) GetType() api.ServiceProviderType {
	if t.GetTypeImpl == nil {
		log.Log.V(logs.DebugLevel).Info("empty impl of GetType called")
		return "TestServiceProvider"
	}
	ret := t.GetTypeImpl()
	log.Log.V(logs.DebugLevel).Info("assigned impl of GetType", "result", ret)
	return ret
}

func (t TestServiceProvider) GetOAuthEndpoint() string {
	if t.GetOauthEndpointImpl == nil {
		log.Log.V(logs.DebugLevel).Info("empty impl of GetOAuthEndpoint called")
		return ""
	}
	ret := t.GetOauthEndpointImpl()
	log.Log.V(logs.DebugLevel).Info("assigned impl of GetOAuthEndpoint called", "result", ret)
	return ret
}

func (t TestServiceProvider) MapToken(ctx context.Context, binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, tokenData *api.Token) (serviceprovider.AccessTokenMapper, error) {
	lg := log.FromContext(ctx)
	if t.MapTokenImpl == nil {
		lg.V(logs.DebugLevel).Info("empty impl of MapToken called")
		return serviceprovider.AccessTokenMapper{}, nil
	}
	ret, err := t.MapTokenImpl(ctx, binding, token, tokenData)
	log.Log.V(logs.DebugLevel).Info("assigned impl of MapToken called", "result", ret, "error", err)
	return ret, err
}

func (t TestServiceProvider) Validate(ctx context.Context, validated serviceprovider.Validated) (serviceprovider.ValidationResult, error) {
	lg := log.FromContext(ctx)
	if t.ValidateImpl == nil {
		lg.V(logs.DebugLevel).Info("empty impl of Validate called")
		return serviceprovider.ValidationResult{}, nil
	}

	ret, err := t.ValidateImpl(ctx, validated)
	log.Log.V(logs.DebugLevel).Info("assigned impl of Validate called", "result", ret, "error", err)
	return ret, err
}

func (t *TestServiceProvider) Reset() {
	t.LookupTokenImpl = nil
	t.GetBaseUrlImpl = nil
	t.OAuthScopesForImpl = nil
	t.GetTypeImpl = nil
	t.GetOauthEndpointImpl = nil
	t.PersistMetadataImpl = nil
	t.CheckRepositoryAccessImpl = nil
	t.MapTokenImpl = nil
	t.ValidateImpl = nil
	if t.CustomizeReset != nil {
		t.CustomizeReset(t)
	}
}

// LookupConcreteToken returns a function that can be used as the TestServiceProvider.LookupTokenImpl that just returns
// a freshly loaded version of the provided token. The token is a pointer to a pointer to the token so that this can
// also support lazily initialized tokens.
func LookupConcreteToken(tokenPointer **api.SPIAccessToken) func(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) ([]api.SPIAccessToken, error) {
	return func(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) ([]api.SPIAccessToken, error) {
		if *tokenPointer == nil {
			return nil, nil
		}

		freshToken := &api.SPIAccessToken{}
		if err := cl.Get(ctx, client.ObjectKeyFromObject(*tokenPointer), freshToken); err != nil {
			log.FromContext(ctx).Error(err, "failed to get the concrete token configured by the test from the cluster", "expected_token", client.ObjectKeyFromObject(*tokenPointer))
			return nil, err
		}
		return []api.SPIAccessToken{*freshToken}, nil
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
