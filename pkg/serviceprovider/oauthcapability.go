package serviceprovider

import (
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type OAuthCapability interface {
	// GetOAuthEndpoint returns the URL of the OAuth initiation. This must point to the SPI oauth service, NOT
	//the service provider itself.
	GetOAuthEndpoint() string

	// OAuthScopesFor translates all the permissions into a list of service-provider-specific scopes. This method
	// is used to compose the OAuth flow URL. There is a generic helper, GetAllScopes, that can be used if all that is
	// needed is just a translation of permissions into scopes.
	OAuthScopesFor(permissions *api.Permissions) []string
}
