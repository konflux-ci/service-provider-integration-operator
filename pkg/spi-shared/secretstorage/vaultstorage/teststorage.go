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

// go:build !release

package vaultstorage

import (
	"github.com/hashicorp/go-hclog"
	kv "github.com/hashicorp/vault-plugin-secrets-kv"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
	vaulthttp "github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/vault"
	vtesting "github.com/mitchellh/go-testing-interface"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"
)

func CreateTestVaultSecretStorage(t vtesting.T) (*vault.TestCluster, *VaultSecretStorage) {
	t.Helper()
	cluster, storage, _, _ := createTestVaultTokenStorage(t, false, nil)
	return cluster, storage
}

func CreateTestVaultSecretStorageWithAuthAndMetrics(t vtesting.T, metricsRegistry *prometheus.Registry) (*vault.TestCluster, *VaultSecretStorage, string, string) {
	t.Helper()
	return createTestVaultTokenStorage(t, true, metricsRegistry)
}

func createTestVaultTokenStorage(t vtesting.T, auth bool, metricsRegistry *prometheus.Registry) (*vault.TestCluster, *VaultSecretStorage, string, string) {
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

		// This needs to be done AFTER configuring the TLS, because ConfigureTLS assumes that the transport is http.Transport
		// and not our round tripper.
		cfg.HttpClient.Transport = httptransport.HttpMetricCollectingRoundTripper{RoundTripper: cfg.HttpClient.Transport}

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

	// we have to be sure that we're passing a true nil to vault storage - not a non-nil interface pointer pointing
	// to a nil struct (yes, I think this is ridiculous, too).
	var nilSafeRegistry prometheus.Registerer
	if metricsRegistry == nil {
		nilSafeRegistry = nil
	} else {
		nilSafeRegistry = metricsRegistry
	}
	return cluster, &VaultSecretStorage{ignoreLoginHandler: !auth, client: client, loginHandler: lh, Config: &VaultStorageConfig{DataPathPrefix: "spi", MetricsRegisterer: nilSafeRegistry}}, roleId, secretId
}
