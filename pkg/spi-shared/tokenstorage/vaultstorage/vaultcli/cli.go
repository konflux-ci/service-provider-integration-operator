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

package vaultcli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage/vaultstorage"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type VaultCliArgs struct {
	VaultHost                       string                       `arg:"--vault-host, env" help:"Mandatory Vault host URL."`
	VaultInsecureTLS                bool                         `arg:"--vault-insecure-tls, env" default:"false" help:"Whether it allows 'insecure' TLS connection to Vault, 'true' is allowing untrusted certificate."`
	VaultAuthMethod                 vaultstorage.VaultAuthMethod `arg:"--vault-auth-method, env" default:"approle" help:"Authentication method to Vault token storage. Options: 'kubernetes', 'approle'."`
	VaultApproleName                string                       `arg:"--vault-approle-name, env" default:"spi-operator" help:"Vault approle name that will be used for Vault authentication."`
	VaultCredentialsSecretNamespace string                       `arg:"--vault-credentials-secret-namespace, env" help:"Namespace where to look for vault credentials secret. If not set, it is taken from POD_NAMESPACE env."`
	VaultKubernetesSATokenFilePath  string                       `arg:"--vault-k8s-sa-token-filepath, env" help:"Used with Vault kubernetes authentication. Filepath to kubernetes ServiceAccount token. When empty, Vault configuration uses default k8s path. No need to set when running in k8s deployment, useful mostly for local development."`
	VaultKubernetesRole             string                       `arg:"--vault-k8s-role, env"  help:"Used with Vault kubernetes authentication. Vault authentication role set for k8s ServiceAccount."`
	VaultDataPathPrefix             string                       `arg:"--vault-data-path-prefix, env" default:"spi" help:"Path prefix in Vault token storage under which all SPI data will be stored. No leading or trailing '/' should be used, it will be trimmed."`
}

// VaultStorageConfigFromCliArgs returns an instance of the VaultStorageConfig with some fields initialized from
// the corresponding CLI arguments. Notably, the VaultStorageConfig.MetricsRegisterer is NOT configured, because this
// cannot be done using just the CLI arguments.
func VaultStorageConfigFromCliArgs(args *VaultCliArgs) (*vaultstorage.VaultStorageConfig, error) {
	if args.VaultCredentialsSecretNamespace == "" {
		namespace, hasNamespace := os.LookupEnv("POD_NAMESPACE")
		if !hasNamespace {
			return nil, fmt.Errorf("couldn't figure out namespace where to find vault tokenstorage creds secret")
		}
		args.VaultCredentialsSecretNamespace = namespace
	}
	return &vaultstorage.VaultStorageConfig{
		Host:                        args.VaultHost,
		AuthType:                    args.VaultAuthMethod,
		Insecure:                    args.VaultInsecureTLS,
		Role:                        args.VaultKubernetesRole,
		ServiceAccountTokenFilePath: args.VaultKubernetesSATokenFilePath,
		AppRoleName:                 args.VaultApproleName,
		CredentialsSecretNamespace:  args.VaultCredentialsSecretNamespace,
		DataPathPrefix:              strings.Trim(args.VaultDataPathPrefix, "/"),
	}, nil
}

func CreateVaultStorage(ctx context.Context, args *VaultCliArgs, client client.Client) (tokenstorage.TokenStorage, error) {
	vaultConfig, errConfig := VaultStorageConfigFromCliArgs(args)
	if errConfig != nil {
		return nil, fmt.Errorf("failed to create vault config: %w", errConfig)
	}
	// use the same metrics registry as the controller-runtime
	vaultConfig.MetricsRegisterer = metrics.Registry
	strg, err := vaultstorage.NewVaultStorage(ctx, vaultConfig, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault token storage: %w", err)
	}
	return strg, nil
}
