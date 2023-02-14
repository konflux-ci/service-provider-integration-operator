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
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
)

// TokenPolicy specifies the policy to use when matching the tokens during the token lookup
type TokenPolicy string

const (
	AnyTokenPolicy   TokenPolicy = "any"
	ExactTokenPolicy TokenPolicy = "exact"
)

type OperatorConfiguration struct {
	config.SharedConfiguration

	// TokenLookupCacheTtl is the time for which the lookup cache results are considered valid
	TokenLookupCacheTtl time.Duration

	// AccessCheckTtl is time after that SPIAccessCheck CR will be deleted.
	AccessCheckTtl time.Duration

	// AccessTokenTtl is time after that AccessToken will be deleted.
	AccessTokenTtl time.Duration

	// AccessTokenBindingTtl is time after that AccessTokenBinding will be deleted.
	AccessTokenBindingTtl time.Duration

	//FileContentRequestTtl is time after that FileContentRequest will be deleted
	FileContentRequestTtl time.Duration

	// The policy to match the token against the binding
	TokenMatchPolicy TokenPolicy

	// The time before a token without data and with no bindings is automatically deleted.
	DeletionGracePeriod time.Duration

	// A maximum file size for file downloading from SCM capabilities supporting providers
	MaxFileDownloadSize int

	// Enable Token Upload controller
	EnableTokenUpload bool
}
