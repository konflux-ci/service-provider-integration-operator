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

package operatorcli

import (
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/cmd"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
)

type OperatorCliArgs struct {
	cmd.CommonCliArgs
	cmd.LoggingCliArgs
	EnableLeaderElection        bool               `arg:"--leader-elect, env" default:"false" help:"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager."`
	TokenMetadataCacheTtl       time.Duration      `arg:"--metadata-cache-ttl, env" default:"1h" help:"The maximum age of token metadata data cache"`
	TokenLifetimeDuration       time.Duration      `arg:"--token-ttl, env" default:"120h" help:"the time after which a token will be automatically deleted in hours, minutes or seconds. Examples:  \"3h\",  \"5h30m40s\" etc"`
	BindingLifetimeDuration     time.Duration      `arg:"--binding-ttl, env" default:"2h" help:"the time after which a token binding will be automatically deleted in hours, minutes or seconds. Examples: \"3h\", \"5h30m40s\" etc"`
	AccessCheckLifetimeDuration time.Duration      `arg:"--access-check-ttl, env" default:"30m" help:"the time after which SPIAccessCheck CR will be deleted by operator"`
	FileRequestLifetimeDuration time.Duration      `arg:"--file-request-ttl, env" default:"30m" help:"the time after which SPIFileContentRequest CR will be deleted by operator"`
	TokenMatchPolicy            config.TokenPolicy `arg:"--token-match-policy, env" default:"any" help:"The policy to match the token against the binding. Options:  'any', 'exact'."`
	DeletionGracePeriod         time.Duration      `arg:"--deletion-grace-period, env" default:"2s" help:"The grace period between a condition for deleting a binding or token is satisfied and the token or binding actually being deleted."`
	MaxFileDownloadSize         int                `arg:"--max-download-size-bytes, env" default:"2097152" help:"A maximum file size in bytes for file downloading from SCM capabilities supporting providers"`
	EnableTokenUpload           bool               `arg:"--enable-token-upload, env" default:"true" help:"Enable Token Upload controller. Enabling this will make possible uploading access token with Secrets."`
}
