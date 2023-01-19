package config

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBaseUrl(t *testing.T) {
	assert.Equal(t, "ht://bla.bol", GetBaseUrl(&url.URL{Scheme: "ht", Host: "bla.bol"}))
	assert.Equal(t, "ht://bla.bol", GetBaseUrl(&url.URL{Scheme: "ht", Host: "bla.bol", Path: "/hello/there"}))
	assert.Equal(t, "", GetBaseUrl(&url.URL{Host: "blabol.sh"}))
	assert.Equal(t, "", GetBaseUrl(&url.URL{Scheme: "bla"}))
	assert.Equal(t, "", GetBaseUrl(nil))
}

func TestGetHostWithScheme(t *testing.T) {
	test := func(name string, inputUrl string, shouldError bool, expectedOutput string) {
		t.Run(name, func(t *testing.T) {
			u, err := GetHostWithScheme(inputUrl)
			if shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, expectedOutput, u)
		})
	}

	test("ok url with path", "https://blabol.com/hello/world", false, "https://blabol.com")
	test("ok url without path", "https://blabol.com", false, "https://blabol.com")
	test("url without scheme", "blab.ol", false, "")
	test("scheme without host", "https://", false, "")
	test("invalid url", ":::", true, "")
}
