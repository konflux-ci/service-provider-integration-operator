package config

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// ServiceProviderDefaults configuration containing default values used to initialize supported service providers
type ServiceProviderDefaults struct {
	SpType   ServiceProviderType
	Endpoint oauth2.Endpoint
	UrlHost  string
}

// all servise provider types we support, including default values
var SupportedServiceProvidersDefaults []ServiceProviderDefaults = []ServiceProviderDefaults{
	{
		SpType:   ServiceProviderTypeGitHub,
		Endpoint: github.Endpoint,
		UrlHost:  GithubSaasHost,
	},
	{
		SpType:   ServiceProviderTypeQuay,
		Endpoint: QuayEndpoint,
		UrlHost:  QuaySaasHost,
	},
	{
		SpType:   ServiceProviderTypeGitLab,
		Endpoint: GitlabEndpoint,
		UrlHost:  GitlabSaasHost,
	},
}
