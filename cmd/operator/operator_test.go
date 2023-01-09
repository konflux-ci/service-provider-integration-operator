package main

import (
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/stretchr/testify/assert"
)

func TestAllServiceProvidersHaveInitializer(t *testing.T) {
	initServiceProviders()

	for _, sp := range config.SupportedServiceProviderTypes {
		initializer, err := initializers.GetInitializer(sp)
		assert.NotNil(t, initializer)
		assert.NoError(t, err)
	}
}
