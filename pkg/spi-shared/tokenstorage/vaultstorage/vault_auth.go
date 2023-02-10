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
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
	"github.com/hashicorp/vault/api/auth/kubernetes"
)

var VaultUnknownAuthMethodError = errors.New("unknown Vault authentication method")

type vaultAuthConfiguration interface {
	prepare(config *VaultStorageConfig) (api.AuthMethod, error)
}

type kubernetesAuth struct{}
type approleAuth struct{}

func prepareAuth(cfg *VaultStorageConfig) (api.AuthMethod, error) {
	var authMethod vaultAuthConfiguration
	if cfg.AuthType == VaultAuthMethodKubernetes {
		authMethod = &kubernetesAuth{}
	} else if cfg.AuthType == VaultAuthMethodApprole {
		authMethod = &approleAuth{}
	} else {
		return nil, VaultUnknownAuthMethodError
	}

	vaultAuth, err := authMethod.prepare(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare auth method '%w'", err)
	}
	return vaultAuth, nil
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
	roleId, err := os.ReadFile(config.RoleIdFilePath)
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
