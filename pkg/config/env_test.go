package config

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvConfiguration(t *testing.T) {
	envBoolTest := func(t *testing.T, varName string, defaultVal bool, funcUnderTest func() bool) {
		var expected bool

		origVal, present := os.LookupEnv(varName)
		if !present {
			expected = defaultVal
		} else {
			var err error
			expected, err = strconv.ParseBool(origVal)
			assert.NoError(t, err)
		}

		actual := funcUnderTest()

		assert.NoError(t, os.Setenv(varName, origVal))

		assert.Equal(t, expected, actual)
	}

	t.Run("controllers", func(t *testing.T) {
		envBoolTest(t, runControllersEnv, runControllersDefault, RunControllers)
	})

	t.Run("webhooks", func(t *testing.T) {
		envBoolTest(t, runWebhooksEnv, runWebhooksDefault, RunWebhooks)
	})
}
