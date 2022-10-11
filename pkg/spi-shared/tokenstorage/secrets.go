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
	"strconv"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/sync"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type secretsTokenStorage struct {
	client.Client
	syncer sync.Syncer
}

var _ TokenStorage = (*secretsTokenStorage)(nil)

// NewSecretsStorage creates a new `TokenStorage` instance using the provided Kubernetes client.
func NewSecretsStorage(cl client.Client) (TokenStorage, error) {
	return &secretsTokenStorage{Client: cl, syncer: sync.New(cl)}, nil
}

func (s secretsTokenStorage) Initialize(_ context.Context) error {
	return nil
}

func (s secretsTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	data := map[string][]byte{
		"username":      []byte(token.Username),
		"token_type":    []byte(token.TokenType),
		"refresh_token": []byte(token.RefreshToken),
		"access_token":  []byte(token.AccessToken),
		"expiry":        []byte(strconv.FormatUint(token.Expiry, 10)),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spi-storage-" + owner.Name,
			Namespace: owner.Namespace,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}

	secret.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: api.GroupVersion.String(),
			Kind:       "SPIAccessToken",
			Name:       owner.Name,
			UID:        owner.UID,
		},
	}

	if err := s.Create(ctx, secret); err != nil {
		if errors.IsAlreadyExists(err) {
			if gerr := s.Client.Get(ctx, client.ObjectKey{Name: owner.Name, Namespace: owner.Namespace}, secret); gerr != nil {
				return fmt.Errorf("error trying to get the secret during the store: %w", gerr)
			}

			secret.Data = data
			secret.Type = corev1.SecretTypeOpaque

			if owner.UID != "" {
				// we're resetting the owner here, because we're taking over the secret from whoever created it before.
				secret.OwnerReferences = []metav1.OwnerReference{
					{
						APIVersion: api.GroupVersion.String(),
						Kind:       "SPIAccessToken",
						Name:       owner.Name,
						UID:        owner.UID,
					},
				}
			}

			err = s.Update(ctx, secret)
			if err != nil {
				err = fmt.Errorf("error trying to udpate the secret during the store: %w", err)
			}
		} else {
			err = fmt.Errorf("error trying to create the secret during the store: %w", err)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (s secretsTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	secret, err := s.getBackingSecret(ctx, owner)
	if err != nil {
		return nil, err
	}

	if secret == nil {
		return nil, nil
	}

	var expiry uint64
	if exp, ok := secret.Data["expiry"]; ok {
		expiry, err = strconv.ParseUint(string(exp), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error trying to parse expiry during get: %w", err)
		}
	}

	return &api.Token{
		Username:     string(secret.Data["username"]),
		AccessToken:  string(secret.Data["access_token"]),
		TokenType:    string(secret.Data["token_type"]),
		RefreshToken: string(secret.Data["refresh_token"]),
		Expiry:       expiry,
	}, nil
}

func (s secretsTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	secret, err := s.getBackingSecret(ctx, owner)
	if err != nil {
		return err
	}
	if secret == nil {
		return nil
	}

	err = s.Client.Delete(ctx, secret)
	if err != nil {
		err = fmt.Errorf("error trying to delete the secret: %w", err)
	}

	return err
}

func (s secretsTokenStorage) getBackingSecret(ctx context.Context, owner *api.SPIAccessToken) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	if err := s.Client.Get(ctx, getSecretKey(owner), secret); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error trying to get the secret: %w", err)
	}

	return secret, nil
}

func getSecretKey(owner *api.SPIAccessToken) client.ObjectKey {
	return client.ObjectKey{
		Name:      "spi-storage-" + owner.Name,
		Namespace: owner.Namespace,
	}
}
