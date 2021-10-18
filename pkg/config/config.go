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
	"fmt"
	"os"
)

const (
	spiUrlEnv              = "SPI_URL"
	bearerTokenFileEnv     = "SPI_BEARER_TOKEN_FILE"
	defaultBearerTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

func SpiUrl() string {
	ret, _ := os.LookupEnv(spiUrlEnv)
	return ret
}

func BearerTokenFile() string {
	ret, ok := os.LookupEnv(bearerTokenFileEnv)
	if !ok {
		return defaultBearerTokenFile
	}

	return ret
}

func ValidateEnv() error {
	if _, ok := os.LookupEnv(spiUrlEnv); !ok {
		return fmt.Errorf("the following environment variables are mandatory: %s", spiUrlEnv)
	}

	return nil
}
