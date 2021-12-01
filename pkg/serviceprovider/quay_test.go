package serviceprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuayServiceProviderUrlFromRepo(t *testing.T) {
	q := Quay{Client: nil}
	spUrl, err := q.GetServiceProviderUrlForRepo("https://quay.io/kachny/husy")
	assert.NoError(t, err)
	assert.Equal(t, "https://quay.io", spUrl)
}

func TestQuayServiceProviderUrlFromWrongRepo(t *testing.T) {
	g := Github{Client: nil}
	_, err := g.GetServiceProviderUrlForRepo("https://quay.ganymedes/kachny/husy")
	assert.Error(t, err)
}

// The lookup is tested in the integration tests because it needs interaction with k8s.
