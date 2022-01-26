package serviceprovider

import api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"

// TokenFilter is a helper interface to implement the ServiceProvider.LookupToken method using the GenericLookup struct.
type TokenFilter interface {
	Matches(binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, spState []byte) (bool, error)
}

// TokenFilterFunc converts a function into the implementation of the TokenFilter interface
type TokenFilterFunc func(binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, spState []byte) (bool, error)

var _ TokenFilter = (TokenFilterFunc)(nil)

func (f TokenFilterFunc) Matches(binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, spState []byte) (bool, error) {
	return f(binding, token, spState)
}
