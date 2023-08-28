package serviceprovider

import (
	"context"

	"golang.org/x/oauth2"
)

type AuthenticatedClientBuilder[C any] interface {
	CreateAuthenticatedClient(context.Context, *oauth2.Token) (*C, error)
}
