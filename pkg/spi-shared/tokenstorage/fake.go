package tokenstorage

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type fakeTokenStorage struct {
	// let's fake it for now
	_s map[client.ObjectKey]api.Token
}

var _ TokenStorage = (*fakeTokenStorage)(nil)

func (v *fakeTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) (string, error) {
	v.storage()[client.ObjectKeyFromObject(owner)] = *token
	return v.GetDataLocation(ctx, owner)
}

func (v *fakeTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	key := client.ObjectKeyFromObject(owner)
	val, ok := v.storage()[key]
	if !ok {
		return nil, nil
	}
	return val.DeepCopy(), nil
}

func (v *fakeTokenStorage) GetDataLocation(ctx context.Context, owner *api.SPIAccessToken) (string, error) {
	return "/spi/" + owner.GetNamespace() + "/" + owner.GetName(), nil
}

func (v *fakeTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	delete(v.storage(), client.ObjectKeyFromObject(owner))
	return nil
}

func (v *fakeTokenStorage) storage() map[client.ObjectKey]api.Token {
	if v._s == nil {
		v._s = map[client.ObjectKey]api.Token{}
	}

	return v._s
}
