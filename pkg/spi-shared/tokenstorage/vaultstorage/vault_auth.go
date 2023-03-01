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
	"errors"
	"fmt"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
	"github.com/hashicorp/vault/api/auth/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	tokenStorageCredentialsLabel           = "spi.appstudio.redhat.com/tokenstorage-credentials"
	tokenStorageCredentialsLabelVaultValue = "vault"
	vaultApproleNameLabel                  = "spi.appstudio.redhat.com/vault-approle"
	vaultCredsSecretApproleRoleIdField     = "role_id"
	vaultCredsSecretApproleSecretIdField   = "secret_id"
)

var VaultUnknownAuthMethodError = errors.New("unknown Vault authentication method")

type vaultAuthConfiguration interface {
	prepare(ctx context.Context) (api.AuthMethod, error)
}

type kubernetesAuth struct {
	config *VaultStorageConfig
}
type approleAuth struct {
	config    *VaultStorageConfig
	k8sClient client.Client
}

func prepareAuth(ctx context.Context, cfg *VaultStorageConfig, k8sClient client.Client) (api.AuthMethod, error) {
	var authMethod vaultAuthConfiguration
	if cfg.AuthType == VaultAuthMethodKubernetes {
		authMethod = &kubernetesAuth{
			config: cfg,
		}
	} else if cfg.AuthType == VaultAuthMethodApprole {
		authMethod = &approleAuth{
			config:    cfg,
			k8sClient: k8sClient,
		}
	} else {
		return nil, VaultUnknownAuthMethodError
	}

	vaultAuth, err := authMethod.prepare(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare auth method '%w'", err)
	}
	return vaultAuth, nil
}

func (a *kubernetesAuth) prepare(ctx context.Context) (api.AuthMethod, error) {
	var auth *kubernetes.KubernetesAuth
	var k8sAuthErr error

	if a.config.ServiceAccountTokenFilePath == "" {
		auth, k8sAuthErr = kubernetes.NewKubernetesAuth(a.config.Role)
	} else {
		auth, k8sAuthErr = kubernetes.NewKubernetesAuth(a.config.Role, kubernetes.WithServiceAccountTokenPath(a.config.ServiceAccountTokenFilePath))
	}

	if k8sAuthErr != nil {
		return nil, fmt.Errorf("error creating kubernetes authenticator: %w", k8sAuthErr)
	}

	return auth, nil
}

func (a *approleAuth) prepare(ctx context.Context) (api.AuthMethod, error) {
	vaultCfgSecrets := &corev1.SecretList{}

	errList := a.k8sClient.List(ctx, vaultCfgSecrets, client.MatchingLabels{
		tokenStorageCredentialsLabel: tokenStorageCredentialsLabelVaultValue,
		vaultApproleNameLabel:        a.config.AppRoleName,
	},
		client.InNamespace(a.config.CredentialsSecretNamespace))
	if errList != nil {
		return nil, fmt.Errorf("failed to list secret with vault credentials: %w", errList)
	}
	if len(vaultCfgSecrets.Items) == 0 || len(vaultCfgSecrets.Items) > 1 {
		return nil, fmt.Errorf("%d secrets with Vault credentials found, but need exactly 1", len(vaultCfgSecrets.Items))
	}

	vaultCredsData := vaultCfgSecrets.Items[0].Data
	roleId, hasRoleId := vaultCredsData[vaultCredsSecretApproleRoleIdField]
	secretId, hasSecretId := vaultCredsData[vaultCredsSecretApproleSecretIdField]
	if !hasRoleId || !hasSecretId {
		return nil, fmt.Errorf("vault approle credentials secret does not have 'roleId' or 'secretId' fields. both are mandatory")
	}

	auth, errNewAuth := approle.NewAppRoleAuth(
		string(roleId),
		&approle.SecretID{
			FromString: string(secretId),
		},
	)
	if errNewAuth != nil {
		return nil, fmt.Errorf("error creating approle authenticator: %w", errNewAuth)
	}
	return auth, nil
}
