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
	"fmt"
	"io/ioutil"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
	"github.com/hashicorp/vault/api/auth/kubernetes"
)

type VaultAuthMethod string

var VaultAuthMethodKubernetes VaultAuthMethod = "kubernetes"
var VaultAuthMethodApprole VaultAuthMethod = "approle"

type vaultAuthConfiguration interface {
	prepare(config *VaultStorageConfig) (api.AuthMethod, error)
}

type kubernetesAuth struct{}
type approleAuth struct{}

func prepareAuth(config *VaultStorageConfig) (api.AuthMethod, error) {
	var authMethod vaultAuthConfiguration
	if config.AuthType == VaultAuthMethodKubernetes {
		authMethod = &kubernetesAuth{}
	} else if config.AuthType == VaultAuthMethodApprole {
		authMethod = &approleAuth{}
	} else {
		return nil, fmt.Errorf("unknown Vault authentication method '%s'", config.AuthType)
	}

	return authMethod.prepare(config)
}

func (a *kubernetesAuth) prepare(config *VaultStorageConfig) (api.AuthMethod, error) {
	var auth *kubernetes.KubernetesAuth
	var k8sAuthErr error

	if config.ServiceAccountTokenFilePath == "" {
		auth, k8sAuthErr = kubernetes.NewKubernetesAuth(config.Role)
	} else {
		auth, k8sAuthErr = kubernetes.NewKubernetesAuth(config.Role, kubernetes.WithServiceAccountTokenPath(config.ServiceAccountTokenFilePath))
	}

	if k8sAuthErr != nil {
		return nil, fmt.Errorf("error creating kubernetes authenticator: %w", k8sAuthErr)
	}

	return auth, nil
}

func (a *approleAuth) prepare(config *VaultStorageConfig) (api.AuthMethod, error) {
	roleId, err := ioutil.ReadFile(config.RoleIdFilePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read vault role id: %w", err)
	}
	secretId := &approle.SecretID{FromFile: config.SecretIdFilePath}

	auth, err := approle.NewAppRoleAuth(string(roleId), secretId)
	if err != nil {
		return nil, fmt.Errorf("error creating approle authenticator: %w", err)
	}
	return auth, nil
}
