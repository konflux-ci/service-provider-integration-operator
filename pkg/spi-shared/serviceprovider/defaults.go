package serviceprovider

import (
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// ServiceProviderDefaults configuration containing default values used to initialize supported service providers
type ServiceProviderDefaults struct {
	SpType   config.ServiceProviderType
	Endpoint oauth2.Endpoint
	UrlHost  string
}

// all servise provider types we support, including default values
var SupportedServiceProvidersDefaults []ServiceProviderDefaults = []ServiceProviderDefaults{
	{
		SpType:   config.ServiceProviderTypeGitHub,
		Endpoint: github.Endpoint,
		UrlHost:  GithubSaasHost,
	},
	{
		SpType:   config.ServiceProviderTypeQuay,
		Endpoint: QuayEndpoint,
		UrlHost:  QuaySaasHost,
	},
	{
		SpType:   config.ServiceProviderTypeGitLab,
		Endpoint: GitlabEndpoint,
		UrlHost:  GitlabSaasHost,
	},
}
