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

package vaultstorage

import (
	"context"
	"testing"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"

	"github.com/stretchr/testify/assert"
)

func init() {
	config.SetupCustomValidations(config.CustomValidationOptions{AllowInsecureURLs: true})
}

func TestVaultLogin_Renewal(t *testing.T) {
	logs.InitDevelLoggers()

	cluster, ts, vaultRoleId, vaultSecretId := CreateTestVaultSecretStorageWithAuthAndMetrics(t, nil)
	defer cluster.Cleanup()

	rootClient := cluster.Cores[0].Client
	_, err := rootClient.Logical().Write("/auth/approle/role/test-role", map[string]interface{}{"token_ttl": "1s"})
	assert.NoError(t, err)

	auth, err := approle.NewAppRoleAuth(vaultRoleId, &approle.SecretID{FromString: vaultSecretId})
	assert.NoError(t, err)

	origToken := ts.client.Token()
	assert.NotNil(t, ts.loginHandler)
	assert.Equal(t, auth, ts.loginHandler.authMethod)
	assert.NoError(t, ts.Initialize(context.TODO()))

	time.Sleep(2 * time.Second)

	assert.NotEqual(t, origToken, ts.client.Token())

	secretId := secretstorage.SecretID{
		Name:      "test",
		Namespace: "test",
	}
	testData := []byte{0, 1, 2}
	assert.NoError(t, ts.Store(context.TODO(), secretId, testData))

	expiredClientCfg := vault.DefaultConfig()
	expiredClientCfg.Address = ts.client.Address()
	assert.NoError(t, expiredClientCfg.ConfigureTLS(&vault.TLSConfig{
		Insecure: true,
	}))
	expiredClient, err := vault.NewClient(expiredClientCfg)
	assert.NoError(t, err)
	expiredClient.SetToken(origToken)

	notRenewingTokenStorage := &VaultSecretStorage{
		client: expiredClient,
		Config: &VaultStorageConfig{
			DataPathPrefix: "spi",
		},
	}
	assert.Error(t, notRenewingTokenStorage.Store(context.TODO(), secretId, testData))
}
