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

package vaultcli

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
