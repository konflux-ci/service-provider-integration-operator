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

//go:build !release
// +build !release

package tokenstorage

import (
	"context"
	"testing"

	kv "github.com/hashicorp/vault-plugin-secrets-kv"
	vaultapi "github.com/hashicorp/vault/api"
	vaulthttp "github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/vault"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type TestTokenStorage struct {
	StoreImpl  func(context.Context, *api.SPIAccessToken, *api.Token) error
	GetImpl    func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error)
	DeleteImpl func(context.Context, *api.SPIAccessToken) error
}

func (t TestTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	if t.StoreImpl == nil {
		return nil
	}

	return t.StoreImpl(ctx, owner, token)
}

func (t TestTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	if t.GetImpl == nil {
		return nil, nil
	}

	return t.GetImpl(ctx, owner)
}

func (t TestTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	if t.DeleteImpl == nil {
		return nil
	}

	return t.DeleteImpl(ctx, owner)
}

var _ TokenStorage = (*TestTokenStorage)(nil)

func CreateTestVaultTokenStorage(t *testing.T) (*vault.TestCluster, TokenStorage) {
	t.Helper()

	coreConfig := &vault.CoreConfig{
		LogicalBackends: map[string]logical.Factory{
			"kv": kv.Factory,
		},
	}
	cluster := vault.NewTestCluster(t, coreConfig, &vault.TestClusterOptions{
		HandlerFunc: vaulthttp.Handler,
	})
	cluster.Start()
	client := cluster.Cores[0].Client

	// Create KV V2 mount
	if err := client.Sys().Mount("spi", &vaultapi.MountInput{
		Type: "kv",
		Options: map[string]string{
			"version": "2",
		},
	}); err != nil {
		t.Fatal(err)
	}

	return cluster, &vaultTokenStorage{Client: client}
}
