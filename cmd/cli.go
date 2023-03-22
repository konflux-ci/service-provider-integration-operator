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

package cmd

import (
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage/awsstorage/awscli"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage/vaultstorage/vaultcli"
)

// LoggingCliArgs define the command line arguments for configuring the logging using Zap.
type LoggingCliArgs struct {
	ZapDevel           bool   `arg:"--zap-devel, env" default:"false" help:"Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn) Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)"`
	ZapEncoder         string `arg:"--zap-encoder, env" default:"" help:"Zap log encoding (‘json’ or ‘console’)"`
	ZapLogLevel        string `arg:"--zap-log-level, env" default:"" help:"Zap Level to configure the verbosity of logging"`
	ZapStackTraceLevel string `arg:"--zap-stacktrace-level, env" default:"" help:"Zap Level at and above which stacktraces are captured"`
	ZapTimeEncoding    string `arg:"--zap-time-encoding, env" default:"iso8601" help:"one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'"`
}

// CommonCliArgs are the command line arguments and environment variable definitions understood by the configuration
// infrastructure shared between the operator and the oauth service.
type CommonCliArgs struct {
	SPIInstanceId     string           `arg:"--spi-instance-id,env" default:"spi-1" help:"ID of this SPI instance. Used to avoid conflicts when multiple SPI instances uses shared resources (e.g. secretstorage)."`
	MetricsAddr       string           `arg:"--metrics-bind-address, env" default:"127.0.0.1:8080" help:"The address the metric endpoint binds to."`
	ProbeAddr         string           `arg:"--health-probe-bind-address, env" default:":8081" help:"The address the probe endpoint binds to."`
	ConfigFile        string           `arg:"--config-file, env" default:"/etc/spi/config.yaml" help:"The location of the configuration file."`
	BaseUrl           string           `arg:"--base-url, env" help:"The externally accessible URL on which the OAuth service is listening. This is used to construct manual-upload and OAuth URLs"`
	AllowInsecureURLs bool             `arg:"--allow-insecure-urls, env" default:"false" help:"Whether is allowed or not to use insecure http URLs in service provider or vault configurations."`
	TokenStorage      TokenStorageType `arg:"--tokenstorage, env" default:"vault" help:"The type of the token storage. Supported types: 'vault', 'aws' (experimental)."`
	vaultcli.VaultCliArgs
	awscli.AWSCliArgs
}

type TokenStorageType string

const (
	VaultTokenStorage TokenStorageType = "vault"
	AWSTokenStorage   TokenStorageType = "aws"
)
