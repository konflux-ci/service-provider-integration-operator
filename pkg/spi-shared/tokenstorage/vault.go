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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/kcp-dev/logicalcluster/v2"

	"github.com/hashicorp/go-hclog"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	vault "github.com/hashicorp/vault/api"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const vaultDataPathFormat = "spi/data/%s/%s"
const vaultDataKcpPathFormat = "spi/data/%s/%s/%s"

type vaultTokenStorage struct {
	*vault.Client
	loginHandler *loginHandler
}

var (
	VaultError             = errors.New("error in Vault")
	corruptedDataError     = errors.New("corrupted data in Vault")
	invalidDataError       = errors.New("invalid data")
	noAuthInfoInVaultError = errors.New("no auth info returned from Vault")
	unexpectedDataError    = errors.New("unexpected data")
	unspecifiedStoreError  = errors.New("failed to store the token, no error but returned nil")
)

type VaultAuthMethod string

const (
	VaultAuthMethodKubernetes VaultAuthMethod = "kubernetes"
	VaultAuthMethodApprole    VaultAuthMethod = "approle"
)

type VaultStorageConfig struct {
	Host     string
	AuthType VaultAuthMethod
	Insecure bool

	Role                        string
	ServiceAccountTokenFilePath string

	RoleIdFilePath   string
	SecretIdFilePath string
}

type VaultCliArgs struct {
	VaultHost                      string          `arg:"--vault-host, env" default:"http://spi-vault:8200" help:"Vault host URL. Default is internal kubernetes service."`
	VaultInsecureTLS               bool            `arg:"--vault-insecure-tls, env" default:"false" help:"Whether is allowed or not insecure vault tls connection."`
	VaultAuthMethod                VaultAuthMethod `arg:"--vault-auth-method, env" default:"approle" help:"Authentication method to Vault token storage. Options: 'kubernetes', 'approle'."`
	VaultApproleRoleIdFilePath     string          `arg:"--vault-roleid-filepath, env" default:"/etc/spi/role_id" help:"Used with Vault approle authentication. Filepath with role_id."`
	VaultApproleSecretIdFilePath   string          `arg:"--vault-secretid-filepath, env" default:"/etc/spi/secret_id" help:"Used with Vault approle authentication. Filepath with secret_id."`
	VaultKubernetesSATokenFilePath string          `arg:"--vault-k8s-sa-token-filepath, env" help:"Used with Vault kubernetes authentication. Filepath to kubernetes ServiceAccount token. When empty, Vault configuration uses default k8s path. No need to set when running in k8s deployment, useful mostly for local development."`
	VaultKubernetesRole            string          `arg:"--vault-k8s-role, env"  help:"Used with Vault kubernetes authentication. Vault authentication role set for k8s ServiceAccount."`
}

func VaultStorageConfigFromCliArgs(args *VaultCliArgs) *VaultStorageConfig {
	return &VaultStorageConfig{
		Host:                        args.VaultHost,
		AuthType:                    args.VaultAuthMethod,
		Insecure:                    args.VaultInsecureTLS,
		Role:                        args.VaultKubernetesRole,
		ServiceAccountTokenFilePath: args.VaultKubernetesSATokenFilePath,
		RoleIdFilePath:              args.VaultApproleRoleIdFilePath,
		SecretIdFilePath:            args.VaultApproleSecretIdFilePath,
	}
}

// NewVaultStorage creates a new `TokenStorage` instance using the provided Vault instance.
func NewVaultStorage(vaultTokenStorageConfig *VaultStorageConfig) (TokenStorage, error) {
	config := vault.DefaultConfig()
	config.Address = vaultTokenStorageConfig.Host
	config.Logger = hclog.Default()
	if vaultTokenStorageConfig.Insecure {
		if err := config.ConfigureTLS(&vault.TLSConfig{
			Insecure: true,
		}); err != nil {
			return nil, fmt.Errorf("error configuring insecure TLS: %w", err)
		}
	}

	vaultClient, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("error creating the client: %w", err)
	}

	authMethod, authErr := prepareAuth(vaultTokenStorageConfig)
	if authErr != nil {
		return nil, fmt.Errorf("error preparing vault authentication: %w", authErr)
	}

	return &vaultTokenStorage{
		Client: vaultClient,
		loginHandler: &loginHandler{
			client:     vaultClient,
			authMethod: authMethod,
		}}, nil
}

func (v *vaultTokenStorage) Initialize(ctx context.Context) error {
	if v.loginHandler == nil {
		log.FromContext(ctx).Info("no login handler configured for Vault - token refresh disabled")
		return nil
	}

	return v.loginHandler.Login(ctx)
}

func (v *vaultTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	data := map[string]interface{}{
		"data": token,
	}
	lg := log.FromContext(ctx)
	path := getVaultPath(ctx, owner)

	s, err := v.Client.Logical().Write(path, data)
	if err != nil {
		return fmt.Errorf("error writing the data to Vault: %w", err)
	}
	if s == nil {
		return unspecifiedStoreError
	}
	for _, w := range s.Warnings {
		lg.Info(w)
	}

	return nil
}

func (v *vaultTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	lg := log.FromContext(ctx)

	path := getVaultPath(ctx, owner)
	secret, err := v.Client.Logical().Read(path)
	if err != nil {
		return nil, fmt.Errorf("error reading the data: %w", err)
	}
	if secret == nil || secret.Data == nil || len(secret.Data) == 0 || secret.Data["data"] == nil {
		lg.V(logs.DebugLevel).Info("no data found in vault at", "path", path)
		return nil, nil
	}
	for _, w := range secret.Warnings {
		lg.Info(w)
	}
	data, dataOk := secret.Data["data"]
	if !dataOk {
		return nil, fmt.Errorf("%w at '%s'", corruptedDataError, path)
	}

	return parseToken(data)
}

func parseToken(data interface{}) (*api.Token, error) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, unexpectedDataError
	}

	token := &api.Token{}
	token.Username = ifaceMapFieldToString(dataMap, "username")
	token.AccessToken = ifaceMapFieldToString(dataMap, "access_token")
	token.TokenType = ifaceMapFieldToString(dataMap, "token_type")
	token.RefreshToken = ifaceMapFieldToString(dataMap, "refresh_token")
	expiry, expiryErr := ifaceMapFieldToUint64(dataMap, "expiry")
	if expiryErr != nil {
		return nil, expiryErr
	}
	token.Expiry = expiry

	return token, nil
}

// ifaceMapFieldToUint64 gets `fieldName` field from `source` map and returns its uint64 value.
// If `fieldName` doesn't exist in map, returns 0. If `fieldName` can't be represented as uint64, return error.
func ifaceMapFieldToUint64(source map[string]interface{}, fieldName string) (uint64, error) {
	if mapVal, ok := source[fieldName]; ok {
		if numberVal, ok := mapVal.(json.Number); ok {
			if val, err := strconv.ParseUint(numberVal.String(), 10, 64); err == nil {
				return val, nil
			} else {
				return 0, fmt.Errorf("%w: invalid '%s' value. '%s' can't be parsed to uint64", invalidDataError, fieldName, numberVal.String())
			}
		}
	}
	return 0, nil
}

// ifaceMapFieldToString gets `fieldName` field from `source` map and returns its string value.
// If `fieldName` doesn't exist in map or can't be returned as string, returns empty string.
func ifaceMapFieldToString(source map[string]interface{}, fieldName string) string {
	if mapVal, ok := source[fieldName]; ok {
		if stringVal, ok := mapVal.(string); ok {
			return stringVal
		}
	}
	return ""
}

func (v *vaultTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	path := getVaultPath(ctx, owner)
	s, err := v.Client.Logical().Delete(path)
	if err != nil {
		return fmt.Errorf("error deleting the data: %w", err)
	}
	log.FromContext(ctx).V(logs.DebugLevel).Info("deleted", "secret", s)
	return nil
}

func getVaultPath(ctx context.Context, owner *api.SPIAccessToken) string {
	if workspace, ok := logicalcluster.ClusterFromContext(ctx); ok {
		return fmt.Sprintf(vaultDataKcpPathFormat, workspace, owner.Namespace, owner.Name)
	} else {
		return fmt.Sprintf(vaultDataPathFormat, owner.Namespace, owner.Name)
	}
}
