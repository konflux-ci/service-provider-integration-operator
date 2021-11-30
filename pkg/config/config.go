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

package config

import (
	"os"
)

const (
	spiUrlEnv              = "SPI_URL"
	bearerTokenFileEnv     = "SPI_BEARER_TOKEN_FILE"
	runControllersEnv      = "RUN_CONTROLLERS"
	runWebhooksEnv         = "RUN_WEBHOOKS"
	defaultBearerTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	tokenEndpointPath      = "/api/v1/token/"

	SPIAccessTokenLinkLabel = "spi.appstudio.redhat.com/linked-access-token"

	runControllersDefault = true
	runWebhooksDefault    = true
)

var (
	spiUrl        = os.Getenv(spiUrlEnv)
	tokenEndpoint = spiUrl + tokenEndpointPath
)

func SpiTokenEndpoint() string {
	return tokenEndpoint
}

func SpiUrl() string {
	return spiUrl
}

func SetSpiUrlForTest(url string) {
	spiUrl = url
	tokenEndpoint = spiUrl + tokenEndpointPath
}

func BearerTokenFile() string {
	ret, ok := os.LookupEnv(bearerTokenFileEnv)
	if !ok {
		return defaultBearerTokenFile
	}

	return ret
}

func RunControllers() bool {
	ret, ok := os.LookupEnv(runControllersEnv)
	if !ok {
		return runControllersDefault
	}

	return "true" == ret
}

func RunWebhooks() bool {
	ret, ok := os.LookupEnv(runWebhooksEnv)
	if !ok {
		return runWebhooksDefault
	}

	return "true" == ret
}

func ValidateEnv() error {
	return nil
}
