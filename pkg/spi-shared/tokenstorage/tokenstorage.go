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

	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

// TokenStorage is a simple interface on top of Kubernetes client to perform CRUD operations on the tokens. This is done
// so that we can provide either secret-based or Vault-based implementation.
type TokenStorage interface {
	Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) (string, error)
	Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error)
	GetDataLocation(ctx context.Context, owner *api.SPIAccessToken) (string, error)
	Delete(ctx context.Context, owner *api.SPIAccessToken) error
}

// NewVaultStorage creates a new `TokenStorage` instance using the provided Kubernetes client.
func NewVaultStorage(role string, vaultHost string, serviceAccountToken string) (TokenStorage, error) {
	config := vault.DefaultConfig()
	config.Address = vaultHost
	client, err := vault.NewClient(config)
	if err != nil {
		return nil, err
	}
	var k8sAuth *auth.KubernetesAuth
	if serviceAccountToken == "" {
		k8sAuth, err = auth.NewKubernetesAuth(role)
	} else {
		k8sAuth, err = auth.NewKubernetesAuth(role, auth.WithServiceAccountTokenPath(serviceAccountToken))
	}
	if err != nil {
		return nil, err
	}

	authInfo, err := client.Auth().Login(context.TODO(), k8sAuth)
	if err != nil {
		return nil, err
	}
	if authInfo == nil {
		return nil, fmt.Errorf("no auth info was returned after login to vault")
	}
	return &vaultTokenStorage{client}, nil
}
