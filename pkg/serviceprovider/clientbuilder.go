package serviceprovider

import (
	"context"

	"golang.org/x/oauth2"
)

type AuthorizedClientBuilder[C any] interface {
	CreateAuthorizedClient(context.Context, *oauth2.Token) (*C, error)
}
