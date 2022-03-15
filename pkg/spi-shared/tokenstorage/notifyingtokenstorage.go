package tokenstorage

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NotifyingTokenStorage is a wrapper around TokenStorage that also automatically creates
// the v1beta1.SPIAccessTokenDataUpdate objects.
type NotifyingTokenStorage struct {
	// Client is the kubernetes client to use to create the v1beta1.SPIAccessTokenDataUpdate objects.
	Client client.Client

	// TokenStorage is the token storage to delegate the actual storage operations to.
	TokenStorage TokenStorage
}

func (n NotifyingTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	if err := n.TokenStorage.Store(ctx, owner, token); err != nil {
		return err
	}

	return n.createDataUpdate(ctx, owner)
}

func (n NotifyingTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	return n.TokenStorage.Get(ctx, owner)
}

func (n NotifyingTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	if err := n.TokenStorage.Delete(ctx, owner); err != nil {
		return err
	}

	return n.createDataUpdate(ctx, owner)
}

func (n NotifyingTokenStorage) createDataUpdate(ctx context.Context, owner *api.SPIAccessToken) error {
	update := &api.SPIAccessTokenDataUpdate{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "token-update-",
			Namespace:    owner.Namespace,
		},
		Spec: api.SPIAccessTokenDataUpdateSpec{
			TokenName: owner.Name,
		},
	}

	return n.Client.Create(ctx, update)
}

var _ TokenStorage = (*NotifyingTokenStorage)(nil)
