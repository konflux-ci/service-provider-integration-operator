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
	"fmt"
	"strconv"

	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const vaultDataPathFormat = "spi/data/%s/%s"

type vaultTokenStorage struct {
	*vault.Client
}

// NewVaultStorage creates a new `TokenStorage` instance using the provided Vault instance.
func NewVaultStorage(role string, vaultHost string, serviceAccountToken string, insecure bool) (TokenStorage, error) {
	config := vault.DefaultConfig()
	config.Address = vaultHost

	if insecure {
		if err := config.ConfigureTLS(&vault.TLSConfig{
			Insecure: true,
		}); err != nil {
			return nil, err
		}
	}

	vaultClient, err := vault.NewClient(config)
	if err != nil {
		return nil, err
	}
	var k8sAuth *auth.KubernetesAuth
	if serviceAccountToken == "" {
		k8sAuth, err = auth.NewKubernetesAuth(role)
	} else {
		k8sAuth, err = auth.NewKubernetesAuth(role, auth.WithServiceAccountTokenPath(serviceAccountToken))
	}
	if err != nil {
		return nil, err
	}

	authInfo, err := vaultClient.Auth().Login(context.TODO(), k8sAuth)
	if err != nil {
		return nil, err
	}
	if authInfo == nil {
		return nil, fmt.Errorf("no auth info was returned after login to vault")
	}
	return &vaultTokenStorage{vaultClient}, nil
}

func (v *vaultTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	data := map[string]interface{}{
		"data": token,
	}
	path := getVaultPath(owner)
	s, err := v.Client.Logical().Write(path, data)
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("failed to store the token, no error but returned nil")
	}
	for _, w := range s.Warnings {
		logf.FromContext(ctx).Info(w)
	}

	return nil
}

func (v *vaultTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	path := getVaultPath(owner)

	secret, err := v.Client.Logical().Read(path)
	if err != nil {
		return nil, err
	}
	if secret == nil || secret.Data == nil || len(secret.Data) == 0 || secret.Data["data"] == nil {
		logf.FromContext(ctx).Info("no data found in vault at", "path", path)
		return nil, nil
	}
	for _, w := range secret.Warnings {
		logf.FromContext(ctx).Info(w)
	}
	data, dataOk := secret.Data["data"]
	if !dataOk {
		return nil, fmt.Errorf("corrupted data in Vault at '%s'", path)
	}

	return parseToken(data)
}

func parseToken(data interface{}) (*api.Token, error) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data")
	}

	token := &api.Token{}
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
				return 0, fmt.Errorf("invalid '%s' value. '%s' can't be parsed to uint64", fieldName, numberVal.String())
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
	s, err := v.Client.Logical().Delete(getVaultPath(owner))
	if err != nil {
		return err
	}
	logf.FromContext(ctx).Info("deleted", "secret", s)
	return nil
}

func getVaultPath(owner *api.SPIAccessToken) string {
	return fmt.Sprintf(vaultDataPathFormat, owner.Namespace, owner.Name)
}
