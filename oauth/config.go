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

package oauth

import (
	"fmt"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
)

type OAuthServiceCliArgs struct {
	config.CommonCliArgs
	config.LoggingCliArgs
	tokenstorage.VaultCliArgs
	ServiceAddr       string `arg:"--service-addr, env" default:"0.0.0.0:8000" help:"Service address to listen on"`
	AllowedOrigins    string `arg:"--allowed-origins, env" default:"https://console.dev.redhat.com,https://prod.foo.redhat.com:1337" help:"Comma-separated list of domains allowed for cross-domain requests"`
	KubeConfig        string `arg:"--kubeconfig, env" default:"" help:""`
	KubeInsecureTLS   bool   `arg:"--kube-insecure-tls, env" default:"false" help:"Whether is allowed or not insecure kubernetes tls connection."`
	AllowInsecureURLs bool   `arg:"--allow-insecure-urls, env" default:"false" help:"Whether is allowed or not to use insecure http URLs in service provider or vault configurations."`
	ApiServer         string `arg:"--api-server, env:API_SERVER" default:"" help:"host:port of the Kubernetes API server to use when handling HTTP requests"`
	ApiServerCAPath   string `arg:"--ca-path, env:API_SERVER_CA_PATH" default:"" help:"the path to the CA certificate to use when connecting to the Kubernetes API server"`
}

type OAuthServiceConfiguration struct {
	config.SharedConfiguration `validate:"required"`
}

func LoadOAuthServiceConfiguration(args OAuthServiceCliArgs) (OAuthServiceConfiguration, error) {
	baseCfg, err := config.LoadFrom(&args.CommonCliArgs)
	if err != nil {
		return OAuthServiceConfiguration{}, fmt.Errorf("failed to load the configuration from file %s: %w", args.ConfigFile, err)
	}

	return OAuthServiceConfiguration{SharedConfiguration: baseCfg}, nil
}
