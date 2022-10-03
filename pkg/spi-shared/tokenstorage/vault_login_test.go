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
	"testing"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"

	"github.com/stretchr/testify/assert"
)

func TestVaultLogin_Renewal(t *testing.T) {

	cluster, ts, roleId, secretId := CreateTestVaultTokenStorageWithAuth(t)
	defer cluster.Cleanup()

	rootClient := cluster.Cores[0].Client
	_, err := rootClient.Logical().Write("/auth/approle/role/test-role", map[string]interface{}{"token_ttl": "1s"})
	assert.NoError(t, err)

	auth, err := approle.NewAppRoleAuth(roleId, &approle.SecretID{FromString: secretId})
	assert.NoError(t, err)

	vts := ts.(*vaultTokenStorage)
	origToken := vts.Token()
	assert.NotNil(t, vts.loginHandler)
	assert.Equal(t, auth, vts.loginHandler.authMethod)
	assert.NoError(t, ts.Initialize(context.TODO()))

	time.Sleep(2 * time.Second)

	assert.NotEqual(t, origToken, vts.Token())

	assert.NoError(t, ts.Store(context.TODO(), testSpiAccessToken, testToken))

	expiredClientCfg := vault.DefaultConfig()
	expiredClientCfg.Address = ts.(*vaultTokenStorage).Client.Address()
	assert.NoError(t, expiredClientCfg.ConfigureTLS(&vault.TLSConfig{
		Insecure: true,
	}))
	expiredClient, err := vault.NewClient(expiredClientCfg)
	assert.NoError(t, err)
	expiredClient.SetToken(origToken)

	notRenewingTokenStorage := &vaultTokenStorage{
		Client: expiredClient,
	}
	assert.Error(t, notRenewingTokenStorage.Store(context.TODO(), testSpiAccessToken, testToken))
}
