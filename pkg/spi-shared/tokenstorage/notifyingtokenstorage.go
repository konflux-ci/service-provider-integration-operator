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

package tokenstorage

import (
	"context"
	"fmt"

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

func (n NotifyingTokenStorage) Initialize(ctx context.Context) error {
	if err := n.TokenStorage.Initialize(ctx); err != nil {
		return fmt.Errorf("wrapped storage error: %w", err)
	}
	return nil
}

func (n NotifyingTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	if err := n.TokenStorage.Store(ctx, owner, token); err != nil {
		return fmt.Errorf("wrapped storage error: %w", err)
	}

	return n.createDataUpdate(ctx, owner)
}

func (n NotifyingTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (token *api.Token, err error) {
	if token, err = n.TokenStorage.Get(ctx, owner); err != nil {
		err = fmt.Errorf("wrapped storage error: %w", err)
	}
	return
}

func (n NotifyingTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	if err := n.TokenStorage.Delete(ctx, owner); err != nil {
		return fmt.Errorf("wrapped storage error: %w", err)
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

	err := n.Client.Create(ctx, update)
	if err != nil {
		return fmt.Errorf("error creating data update: %w", err)
	}
	return nil
}

var _ TokenStorage = (*NotifyingTokenStorage)(nil)
