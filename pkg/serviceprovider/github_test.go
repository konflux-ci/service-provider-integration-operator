package serviceprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitHubServiceProviderUrlFromRepo(t *testing.T) {
	g := Github{Client: nil}
	spUrl, err := g.GetServiceProviderUrlForRepo("https://github.com/kachny/husy")
	assert.NoError(t, err)
	assert.Equal(t, "https://github.com", spUrl)
}

func TestGitHubServiceProviderUrlFromWrongRepo(t *testing.T) {
	g := Github{Client: nil}
	_, err := g.GetServiceProviderUrlForRepo("https://githul.com/kachny/husy")
	assert.Error(t, err)
}

// The lookup is tested in the integration tests because it needs interaction with k8s.
