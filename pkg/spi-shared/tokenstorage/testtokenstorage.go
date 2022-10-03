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

	"github.com/hashicorp/go-hclog"
	kv "github.com/hashicorp/vault-plugin-secrets-kv"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
	vaulthttp "github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/vault"
	vtesting "github.com/mitchellh/go-testing-interface"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type TestTokenStorage struct {
	InitializeImpl func(context.Context) error
	StoreImpl      func(context.Context, *api.SPIAccessToken, *api.Token) error
	GetImpl        func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error)
	DeleteImpl     func(context.Context, *api.SPIAccessToken) error
}

func (t TestTokenStorage) Initialize(ctx context.Context) error {
	if t.InitializeImpl == nil {
		return nil
	}

	return t.InitializeImpl(ctx)
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

func CreateTestVaultTokenStorage(t vtesting.T) (*vault.TestCluster, TokenStorage) {
	t.Helper()
	cluster, storage, _, _ := createTestVaultTokenStorage(t, false)
	return cluster, storage
}

func CreateTestVaultTokenStorageWithAuth(t vtesting.T) (*vault.TestCluster, TokenStorage, string, string) {
	t.Helper()
	return createTestVaultTokenStorage(t, true)
}

func createTestVaultTokenStorage(t vtesting.T, auth bool) (*vault.TestCluster, TokenStorage, string, string) {
	t.Helper()

	coreConfig := &vault.CoreConfig{
		LogicalBackends: map[string]logical.Factory{
			"kv": kv.Factory,
		},
	}

	clusterCfg := &vault.TestClusterOptions{
		HandlerFunc: vaulthttp.Handler,
		NumCores:    1,
		Logger:      hclog.Default().With("vault", "cluster"),
	}

	cluster := vault.NewTestCluster(t, coreConfig, clusterCfg)
	cluster.Start()

	// the client that we're returning to the caller
	var client *vaultapi.Client

	if auth {
		cfg := vaultapi.DefaultConfig()
		cfg.Address = cluster.Cores[0].Client.Address()
		cfg.Logger = hclog.Default().With("vault", "authenticating-client")

		var err error
		if err = cfg.ConfigureTLS(&vaultapi.TLSConfig{
			Insecure: true,
		}); err != nil {
			t.Fatal(err)
		}

		client, err = vaultapi.NewClient(cfg)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		client = cluster.Cores[0].Client
		client.SetLogger(hclog.Default().With("vault", "root-client"))
	}

	// we're going to have to do the setup using the privileged client
	rootClient := cluster.Cores[0].Client

	// Create KV V2 mount
	if err := rootClient.Sys().Mount("spi", &vaultapi.MountInput{
		Type: "kv",
		Options: map[string]string{
			"version": "2",
		},
	}); err != nil {
		t.Fatal(err)
	}

	var roleId, secretId string

	var lh *loginHandler

	if auth {
		if err := rootClient.Sys().EnableAuthWithOptions("approle", &vaultapi.EnableAuthOptions{
			Type: "approle",
		}); err != nil {
			t.Fatal(err)
		}

		if err := rootClient.Sys().PutPolicy("test-policy", `path "/spi/*" { capabilities = ["create", "read", "update", "patch", "delete", "list"] }`); err != nil {
			t.Fatal(err)
		}

		if _, err := rootClient.Logical().Write("/auth/approle/role/test-role", map[string]interface{}{
			"token_policies": "test-policy",
		}); err != nil {
			t.Fatal(err)
		}

		resp, err := rootClient.Logical().Read("/auth/approle/role/test-role/role-id")
		if err != nil {
			t.Fatal(err)
		}
		roleId = resp.Data["role_id"].(string)

		resp, err = rootClient.Logical().Write("/auth/approle/role/test-role/secret-id", nil)
		if err != nil {
			t.Fatal(err)
		}
		secretId = resp.Data["secret_id"].(string)

		approleAuth, err := approle.NewAppRoleAuth(roleId, &approle.SecretID{FromString: secretId})
		if err != nil {
			t.Fatal(err)
		}

		lh = &loginHandler{
			client:     client,
			authMethod: approleAuth,
		}
	}

	return cluster, &vaultTokenStorage{Client: client, loginHandler: lh}, roleId, secretId
}
