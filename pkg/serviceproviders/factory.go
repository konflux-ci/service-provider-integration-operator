package serviceproviders

import (
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/github"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/quay"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
)

// KnownInitializers returns a map of service provider initializers known at compile time. The serviceprovider.Factory.Initializers
// should be set to this value under normal circumstances.
func KnownInitializers() map[config.ServiceProviderType]serviceprovider.Initializer {
	return map[config.ServiceProviderType]serviceprovider.Initializer{
		config.ServiceProviderTypeGitHub: github.Initializer,
		config.ServiceProviderTypeQuay:   quay.QuayInitializer,
	}
}
