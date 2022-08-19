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
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
)

// TokenPolicy specifies the policy to use when matching the tokens during the token lookup
type TokenPolicy string

const (
	AnyTokenPolicy   TokenPolicy = "any"
	ExactTokenPolicy TokenPolicy = "exact"
)

type OperatorCliArgs struct {
	config.SharedCliArgs
	TokenMetadataCacheTtl       string      `arg:"--metadata-cache-ttl, env" default:"1h" help:"The maximum age of token metadata data cache"`
	TokenLifetimeDuration       string      `arg:"--token-ttl, env" default:"120h" help:"the time after which a token will be automatically deleted in hours, minutes or seconds. Examples:  \"3h\",  \"5h30m40s\" etc"`
	BindingLifetimeDuration     string      `arg:"--binding-ttl, env" default:"2h" help:"the time after which a token binding will be automatically deleted in hours, minutes or seconds. Examples: \"3h\", \"5h30m40s\" etc"`
	AccessCheckLifetimeDuration string      `arg:"--access-check-ttl, env" default:"30m" help:"the time after which SPIAccessCheck CR will be deleted by operator"`
	TokenMatchPolicy            TokenPolicy `arg:"--token-match-policy, env" default:"any" help:"The policy to match the token against the binding. Options:  'any', 'exact'."`
	ApiExportName               string      `arg:"--kcp-api-export-name, env" default:"spi" help:"SPI ApiExport name used in KCP environment to configure controller with virtual workspace."`
}

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

	// The policy to match the token against the binding
	TokenMatchPolicy TokenPolicy
}

func LoadFrom(args *OperatorCliArgs) (OperatorConfiguration, error) {
	baseCfg, err := config.LoadFrom(args.ConfigFile)
	if err != nil {
		return OperatorConfiguration{}, fmt.Errorf("failed to load the configuration file from %s: %w", args.ConfigFile, err)
	}
	ret := OperatorConfiguration{SharedConfiguration: baseCfg}
	setOrDefaultTokenPolicy(&ret.TokenMatchPolicy, args.TokenMatchPolicy, AnyTokenPolicy)

	var parseErr error

	ret.TokenLookupCacheTtl, parseErr = config.ParseDuration(args.TokenMetadataCacheTtl, "1h")
	if parseErr != nil {
		return ret, fmt.Errorf("failed to parse metadata-cache-ttl: %w", parseErr)
	}

	ret.AccessCheckTtl, parseErr = config.ParseDuration(args.AccessCheckLifetimeDuration, "30m")
	if parseErr != nil {
		return ret, fmt.Errorf("failed to parse access-check-ttl: %w", parseErr)
	}

	ret.AccessTokenTtl, parseErr = config.ParseDuration(args.TokenLifetimeDuration, "120h")
	if parseErr != nil {
		return ret, fmt.Errorf("failed to parse token-ttl: %w", parseErr)
	}

	ret.AccessTokenBindingTtl, parseErr = config.ParseDuration(args.BindingLifetimeDuration, "2h")
	if parseErr != nil {
		return ret, fmt.Errorf("failed to parse binding-ttl: %w", parseErr)
	}

	return ret, nil
}

func setOrDefaultTokenPolicy(target *TokenPolicy, source TokenPolicy, defaultValue TokenPolicy) {
	var val TokenPolicy
	if source == val { // is empty
		val = defaultValue
	} else {
		val = source
	}
	*target = val
}
