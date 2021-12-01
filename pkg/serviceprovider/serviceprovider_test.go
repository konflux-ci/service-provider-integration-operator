package serviceprovider

import (
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestDetectsGithubUrls(t *testing.T) {
	spt, err := TypeFromURL("https://github.com")
	assert.NoError(t, err)
	assert.Equal(t, api.ServiceProviderTypeGitHub, spt)
}

func TestDetectsQuayUrls(t *testing.T) {
	spt, err := TypeFromURL("https://quay.io")
	assert.NoError(t, err)
	assert.Equal(t, api.ServiceProviderTypeQuay, spt)
}

func TestFailsOnUnknownProviderUrl(t *testing.T) {
	_, err := TypeFromURL("https://over.the.rainbow")
	assert.Error(t, err)
}
