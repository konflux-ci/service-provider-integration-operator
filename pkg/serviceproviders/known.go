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

package serviceproviders

import (
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/github"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/hostcredentials"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/quay"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
)

// KnownInitializers returns a map of service provider initializers known at compile time. The serviceprovider.Factory.Initializers
// should be set to this value under normal circumstances.
//
// NOTE: This is pulled out of the serviceprovider package to avoid a circular dependency between it and
// the implementation packages.
func KnownInitializers() map[config.ServiceProviderType]serviceprovider.Initializer {
	return map[config.ServiceProviderType]serviceprovider.Initializer{
		config.ServiceProviderTypeGitHub:          github.Initializer,
		config.ServiceProviderTypeQuay:            quay.Initializer,
		config.ServiceProviderTypeHostCredentials: hostcredentials.Initializer,
	}
}
