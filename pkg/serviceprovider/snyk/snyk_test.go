package snyk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURLProbe_Examine(t *testing.T) {
	probe := snykProbe{}
	test := func(t *testing.T, url string, expectedMatch bool) {
		baseUrl, err := probe.Examine(nil, url)
		expectedBaseUrl := ""
		if expectedMatch {
			expectedBaseUrl = "https://snyk.io"
		}

		assert.NoError(t, err)
		assert.Equal(t, expectedBaseUrl, baseUrl)
	}

	test(t, "https://snyk.io", true)
	test(t, "https://api.snyk.io", true)
	test(t, "https://github.com/name/repo", false)
	test(t, "quay.io/name/repo", false)
}
