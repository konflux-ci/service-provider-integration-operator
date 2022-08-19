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
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/vault/api"

	"github.com/hashicorp/vault/api/auth/approle"
	"github.com/hashicorp/vault/api/auth/kubernetes"
	"github.com/stretchr/testify/assert"
)

func TestPrepareKubernetesAuth(t *testing.T) {
	saTokenFile := createFile(t, "satoken", "anything")
	defer os.Remove(saTokenFile)

	authMethod, err := prepareAuth(
		&VaultStorageConfig{
			AuthType:                    VaultAuthMethodKubernetes,
			Role:                        "test-role",
			ServiceAccountTokenFilePath: saTokenFile,
		})

	assert.NoError(t, err)
	assert.IsType(t, &kubernetes.KubernetesAuth{}, authMethod)
}

func TestPrepareApproleAuth(t *testing.T) {
	roleIdFile := createFile(t, "role_id", "anything")
	defer os.Remove(roleIdFile)

	secretIdFile := createFile(t, "secret_id", "anything")
	defer os.Remove(secretIdFile)

	authMethod, err := prepareAuth(
		&VaultStorageConfig{
			AuthType:         VaultAuthMethodApprole,
			RoleIdFilePath:   roleIdFile,
			SecretIdFilePath: secretIdFile,
		})

	assert.NoError(t, err)
	assert.IsType(t, &approle.AppRoleAuth{}, authMethod)
}

func TestFail(t *testing.T) {
	checkFailed := func(t *testing.T, authMethod api.AuthMethod, err error) {
		assert.Error(t, err)
		assert.Nil(t, authMethod)
	}

	t.Run("unknown auth method", func(t *testing.T) {
		authMethod, err := prepareAuth(
			&VaultStorageConfig{
				AuthType: "blabol",
			})
		checkFailed(t, authMethod, err)
	})

	t.Run("approle no roleid file", func(t *testing.T) {
		authMethod, err := prepareAuth(
			&VaultStorageConfig{
				AuthType: VaultAuthMethodApprole,
			})
		checkFailed(t, authMethod, err)
	})

	t.Run("approle empty roleid file", func(t *testing.T) {
		roleIdFile := createFile(t, "role_id", "")
		defer os.Remove(roleIdFile)

		authMethod, err := prepareAuth(
			&VaultStorageConfig{
				AuthType:       VaultAuthMethodApprole,
				RoleIdFilePath: roleIdFile,
			})
		checkFailed(t, authMethod, err)
	})

	t.Run("kubernetes no token sa file", func(t *testing.T) {
		authMethod, err := prepareAuth(
			&VaultStorageConfig{
				AuthType:                    VaultAuthMethodKubernetes,
				ServiceAccountTokenFilePath: "blabol",
			})
		checkFailed(t, authMethod, err)
	})
}

func createFile(t *testing.T, path string, content string) string {
	file, err := os.CreateTemp(os.TempDir(), path)
	assert.NoError(t, err)

	assert.NoError(t, ioutil.WriteFile(file.Name(), []byte(content), fs.ModeExclusive))

	filePath, err := filepath.Abs(file.Name())
	assert.NoError(t, err)

	return filePath
}
