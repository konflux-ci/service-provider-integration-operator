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
	"encoding/json"
	"errors"
	"fmt"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
)

// TokenStorage is a simple interface on top of Kubernetes client to perform CRUD operations on the tokens. This is done
// so that we can provide either secret-based or Vault-based implementation.
type TokenStorage interface {
	Initialize(ctx context.Context) error
	Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error
	Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error)
	Delete(ctx context.Context, owner *api.SPIAccessToken) error
}

// DefaultTokenStorage is the default implementation of the TokenStorage interface that uses the underlying SecretStorage
// for actually storing the data.
type DefaultTokenStorage struct {
	// SecretStorage is the underlying storage that handled the storing of the tokens
	SecretStorage secretstorage.SecretStorage
	// Serializer is a function to turn the tokens into byte arrays. You can use JSONSerializer if it fits your purpose.
	Serializer func(*api.Token) ([]byte, error)
	// Deserializer is a function to turn byte arrays back into token objects. You can use JSONDeserializer if it fits your purpose.
	Deserializer func([]byte, *api.Token) error

	spiInstanceId string
}

// JSONSerializer is a thin wrapper around Marshal function of encoding/json.
func JSONSerializer(token *api.Token) ([]byte, error) {
	bytes, err := json.Marshal(token)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize token data: %w", err)
	}
	return bytes, nil
}

// JSONDeserializer is a thin wrapper around Unmarshal function of encoding/json.
func JSONDeserializer(data []byte, token *api.Token) error {
	if err := json.Unmarshal(data, token); err != nil {
		return fmt.Errorf("failed to deserialize token data: %w", err)
	}
	return nil
}

// Delete implements TokenStorage
func (s *DefaultTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	if err := s.SecretStorage.Delete(ctx, s.toSecretID(owner)); err != nil && errors.Is(err, secretstorage.NotFoundError) {
		return fmt.Errorf("failed to delete the token data: %w", err)
	}
	return nil
}

// Get implements TokenStorage
func (s *DefaultTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	data, err := s.SecretStorage.Get(ctx, s.toSecretID(owner))
	if errors.Is(err, secretstorage.NotFoundError) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get the token data: %w", err)
	}

	t, err := s.toToken(data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize the token data: %w", err)
	}

	return t, nil
}

// Initialize implements TokenStorage
func (s *DefaultTokenStorage) Initialize(ctx context.Context) error {
	if err := s.SecretStorage.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize the underlying secret storage: %w", err)
	}

	s.spiInstanceId = fmt.Sprint(ctx.Value(config.SPIInstanceIdContextKey))

	return nil
}

// Store implements TokenStorage
func (s *DefaultTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	data, err := s.toData(token)
	if err != nil {
		return fmt.Errorf("failed to serialize the token data: %w", err)
	}

	if err = s.SecretStorage.Store(ctx, s.toSecretID(owner), data); err != nil {
		return fmt.Errorf("failed to store the token data: %w", err)
	}

	return nil
}

func (s *DefaultTokenStorage) toToken(data []byte) (*api.Token, error) {
	token := &api.Token{}
	if err := s.Deserializer(data, token); err != nil {
		return nil, fmt.Errorf("failed to deserialize the data to api.Token: %w", err)
	}

	return token, nil
}

func (s *DefaultTokenStorage) toData(t *api.Token) ([]byte, error) {
	data, err := s.Serializer(t)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize the api.Token: %w", err)
	}

	return data, nil
}

var _ TokenStorage = (*DefaultTokenStorage)(nil)

func (s *DefaultTokenStorage) toSecretID(t *api.SPIAccessToken) secretstorage.SecretID {
	return secretstorage.SecretID{
		Name:          t.Name,
		Namespace:     t.Namespace,
		SpiInstanceId: s.spiInstanceId,
	}
}
