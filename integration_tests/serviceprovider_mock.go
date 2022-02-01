package integrationtests

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestServiceProvider is an implementation of the serviceprovider.ServiceProvider interface that can be modified by
// supplying custom implementations of each of the interface method. It provides dummy implementations of them, too, so
// that no null pointer dereferences should occur under normal operation.
type TestServiceProvider struct {
	LookupTokenImpl       func(context.Context, client.Client, *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error)
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
}
