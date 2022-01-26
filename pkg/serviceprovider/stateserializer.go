package serviceprovider

import (
	"context"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

// StateSerializer is a function that converts a token into some string representing its state.
type StateSerializer interface {
	Serialize(ctx context.Context, token *api.SPIAccessToken) ([]byte, error)
}

type StateSerializerFunc func(ctx context.Context, token *api.SPIAccessToken) ([]byte, error)

var _ StateSerializer = (StateSerializerFunc)(nil)

func (f StateSerializerFunc) Serialize(ctx context.Context, token *api.SPIAccessToken) ([]byte, error) {
	return f(ctx, token)
}
