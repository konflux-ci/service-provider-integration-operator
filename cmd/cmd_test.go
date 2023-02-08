package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateTokenStorage(t *testing.T) {
	t.Run("unsupported type", func(t *testing.T) {
		var blabol TokenStorageType = "eh"

		strg, err := InitTokenStorage(context.TODO(), &CommonCliArgs{TokenStorage: blabol})

		assert.Nil(t, strg)
		assert.Error(t, err)
		assert.ErrorIs(t, err, errUnsupportedTokenStorage)
	})

	t.Run("fail vault init", func(t *testing.T) {
		strg, err := InitTokenStorage(context.TODO(), &CommonCliArgs{TokenStorage: VaultTokenStorage})

		assert.Nil(t, strg)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "vault")
	})
}
