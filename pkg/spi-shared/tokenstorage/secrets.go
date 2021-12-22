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

func (s secretsTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) (string, error) {
	data := map[string][]byte{
		"token_type":    []byte(token.TokenType),
		"refresh_token": []byte(token.RefreshToken),
		"access_token":  []byte(token.AccessToken),
		"expiry":        []byte(strconv.FormatUint(token.Expiry, 10)),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      owner.Name,
			Namespace: owner.Namespace,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}

	if owner.UID != "" {
		secret.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: api.GroupVersion.String(),
				Kind:       "SPIAccessToken",
				Name:       owner.Name,
				UID:        owner.UID,
			},
		}
	}

	if err := s.Create(ctx, secret); err != nil {
		if errors.IsAlreadyExists(err) {
			if gerr := s.Client.Get(ctx, client.ObjectKey{Name: owner.Name, Namespace: owner.Namespace}, secret); gerr != nil {
				return "", gerr
			}

			secret.Data = data
			secret.Type = corev1.SecretTypeOpaque

			err = s.Update(ctx, secret)
		}

		if err != nil {
			return "", err
		}
	}

	return owner.Namespace + ":" + owner.Name, nil
}

func (s secretsTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, client.ObjectKey{Name: owner.Name, Namespace: owner.Namespace}, secret); err != nil {
		return nil, err
	}

	expiry, err := strconv.ParseUint(string(secret.Data["expiry"]), 10, 64)
	if err != nil {
		return nil, err
	}

	return &api.Token{
		AccessToken:  string(secret.Data["access_token"]),
		TokenType:    string(secret.Data["token_type"]),
		RefreshToken: string(secret.Data["refresh_token"]),
		Expiry:       expiry,
	}, nil
}

func (s secretsTokenStorage) GetDataLocation(ctx context.Context, owner *api.SPIAccessToken) (string, error) {
	if _, err := s.Get(ctx, owner); err != nil {
		if errors.IsNotFound(err) {
			return "", nil
		} else {
			return "", err
		}
	}

	return owner.Namespace + ":" + owner.Name, nil
}

func (s secretsTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	secret := &corev1.Secret{}
	if err := s.Client.Get(ctx, client.ObjectKey{Name: owner.Name, Namespace: owner.Namespace}, secret); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return s.Client.Delete(ctx, secret)
}
