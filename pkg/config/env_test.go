//
// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
}
