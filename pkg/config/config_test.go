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
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/stretchr/testify/assert"
)

func TestDefaults(t *testing.T) {
	configFileContent := `
`
	cfgFilePath := createFile(t, "config", configFileContent)
	defer os.Remove(cfgFilePath)

	cfg, err := LoadFrom(&OperatorCliArgs{CommonCliArgs: config.CommonCliArgs{ConfigFile: cfgFilePath}})
	assert.NoError(t, err)

	assert.Equal(t, cfg.AccessCheckTtl, 30*time.Minute)
	assert.Equal(t, cfg.TokenLookupCacheTtl, 1*time.Hour)
	assert.Equal(t, cfg.AccessTokenTtl, 120*time.Hour)
	assert.Equal(t, cfg.TokenMatchPolicy, AnyTokenPolicy)
	assert.Equal(t, cfg.AccessTokenBindingTtl, 2*time.Hour)
}

func createFile(t *testing.T, path string, content string) string {
	file, err := os.CreateTemp(os.TempDir(), path)
	assert.NoError(t, err)

	assert.NoError(t, ioutil.WriteFile(file.Name(), []byte(content), fs.ModeExclusive))

	filePath, err := filepath.Abs(file.Name())
	assert.NoError(t, err)

	return filePath
}
