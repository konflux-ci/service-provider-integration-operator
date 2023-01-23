package serviceprovider

import (
	"context"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type RefreshTokenNotSupportedError struct {
}

func (f RefreshTokenNotSupportedError) Error() string {
	return "service provider does not support token refreshing"
}

// RefreshTokenCapability indicates an ability of given SCM provider to refresh issued OAuth access tokens.
type RefreshTokenCapability interface {
	// RefreshToken requests new access token from the service provider using refresh token as authorization.
	// This invalidates the old access token and refresh token
	RefreshToken(ctx context.Context, token *api.Token, clientId string, clientSecret string) (*api.Token, error)
}
