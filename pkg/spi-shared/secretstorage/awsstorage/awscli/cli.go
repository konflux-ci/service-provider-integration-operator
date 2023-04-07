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

package awscli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go/logging"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage/awsstorage"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type AWSCliArgs struct {
	ConfigFile      string `arg:"--aws-config-filepath, env: AWS_CONFIG_FILE" default:"/etc/spi/aws/config" help:"Filepath to AWS configuration file."`
	CredentialsFile string `arg:"--aws-credentials-filepath, env: AWS_CREDENTIALS_FILE" default:"/etc/spi/aws/credentials" help:"Filepath to AWS credentials file."`
}

var (
	errInvalidCliArgs = errors.New("invalid cli args")
)

func NewAwsSecretStorage(ctx context.Context, args *AWSCliArgs) (secretstorage.SecretStorage, error) {
	log.FromContext(ctx).Info("creating aws client")
	cfg, err := configFromCliArgs(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS secretmanager configuration: %w", err)
	}

	return &awsstorage.AwsSecretStorage{
		Config: cfg,
	}, nil
}

func configFromCliArgs(ctx context.Context, args *AWSCliArgs) (*aws.Config, error) {
	log.FromContext(ctx).V(logs.DebugLevel).Info("creating AWS config")

	if !validateCliArgs(ctx, args) {
		return nil, errInvalidCliArgs
	}

	awsLogger := logging.NewStandardLogger(os.Stdout)

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigFiles([]string{args.ConfigFile}),
		config.WithSharedCredentialsFiles([]string{args.CredentialsFile}),
		config.WithLogConfigurationWarnings(true),
		config.WithLogger(awsLogger))
	if err != nil {
		return nil, fmt.Errorf("failed to create aws tokenstorage configuration: %w", err)
	}
	return &cfg, nil
}

func validateCliArgs(ctx context.Context, args *AWSCliArgs) bool {
	lg := log.FromContext(ctx)
	ok := true

	if args.ConfigFile == "" {
		ok = false
		lg.Info("aws config file config option missing")
	} else if _, err := os.Stat(args.ConfigFile); errors.Is(err, os.ErrNotExist) {
		ok = false
		lg.Info("aws config file does not exist", "configfile", args.ConfigFile)
	}

	if args.CredentialsFile == "" {
		ok = false
		lg.Info("aws credentials file config option missing")
	} else if _, err := os.Stat(args.CredentialsFile); errors.Is(err, os.ErrNotExist) {
		ok = false
		lg.Info("aws credentials file does not exist", "credentialsfile", args.CredentialsFile)
	}

	return ok
}
