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
	"errors"
	"fmt"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
)

// TokenStorage is an interface similar to SecretStorage that has historically been used to work with tokens.
// New storage types are built on top of secretstorage.TypedSecretStorage that provides the same interface
// generically.
type TokenStorage interface {
	Initialize(ctx context.Context) error
	Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error
	Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error)
	Delete(ctx context.Context, owner *api.SPIAccessToken) error
}

// NewJSONSerializingTokenStorage is a convenience function to construct a TokenStorage instance
// based on the provided SecretStorage and serializing the data to JSON for persistence.
// The returned object is an instance of DefaultTokenStorage.
func NewJSONSerializingTokenStorage(secretStorage secretstorage.SecretStorage) TokenStorage {
	return &DefaultTokenStorage{
		SecretStorage: &secretstorage.DefaultTypedSecretStorage[api.SPIAccessToken, api.Token]{
			DataTypeName:  "token",
			SecretStorage: secretStorage,
			ToID:          secretstorage.ObjectToID[*api.SPIAccessToken],
			Serialize:     secretstorage.SerializeJSON[api.Token],
			Deserialize:   secretstorage.DeserializeJSON[api.Token],
		},
	}
}

// DefaultTokenStorage is the default implementation of the TokenStorage interface that uses the underlying SecretStorage
// for actually storing the data.
// It honors the original behavior of the TokenStorage interface that used to return nil instead
// of throwing NotFoundError when encountering non-existent records in Get and Delete.
type DefaultTokenStorage struct {
	// SecretStorage is the underlying storage that handled the storing of the tokens
	SecretStorage secretstorage.TypedSecretStorage[api.SPIAccessToken, api.Token]
}

// Delete implements TokenStorage
func (s *DefaultTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	if err := s.SecretStorage.Delete(ctx, owner); err != nil && errors.Is(err, secretstorage.NotFoundError) {
		return fmt.Errorf("failed to delete the token data: %w", err)
	}
	return nil
}

// Get implements TokenStorage
func (s *DefaultTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	t, err := s.SecretStorage.Get(ctx, owner)
	if errors.Is(err, secretstorage.NotFoundError) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get the token data: %w", err)
	}

	return t, nil
}

// Initialize implements TokenStorage
func (s *DefaultTokenStorage) Initialize(ctx context.Context) error {
	if err := s.SecretStorage.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize the underlying secret storage: %w", err)
	}

	return nil
}

// Store implements TokenStorage
func (s *DefaultTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	if err := s.SecretStorage.Store(ctx, owner, token); err != nil {
		return fmt.Errorf("failed to store the token data: %w", err)
	}

	return nil
}

var _ TokenStorage = (*DefaultTokenStorage)(nil)
