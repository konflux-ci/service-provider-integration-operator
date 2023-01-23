package tokenstorage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigDataPathPrefix(t *testing.T) {
	test := func(cliPrefix, expectedPrefix string) {
		t.Run("simple single path level", func(t *testing.T) {
			config := VaultStorageConfigFromCliArgs(&VaultCliArgs{VaultDataPathPrefix: cliPrefix})
			assert.Equal(t, expectedPrefix, config.DataPathPrefix)
		})
	}

	test("spi", "spi")
	test("spi/at/some/deep/paath", "spi/at/some/deep/paath")
	test("/cut/leading/slash", "cut/leading/slash")
	test("cut/trailing/slash/", "cut/trailing/slash")
	test("/cut/both/slashes/", "cut/both/slashes")
	test("/spi/", "spi")
}
