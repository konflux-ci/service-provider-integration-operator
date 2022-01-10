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
	runControllersEnv = "RUN_CONTROLLERS"
	runWebhooksEnv    = "RUN_WEBHOOKS"

	SPIAccessTokenLinkLabel = "spi.appstudio.redhat.com/linked-access-token"

	runControllersDefault = true
	runWebhooksDefault    = true
)

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
