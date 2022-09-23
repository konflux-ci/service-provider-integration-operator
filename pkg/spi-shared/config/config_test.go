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

	"github.com/stretchr/testify/assert"
)

func TestRead(t *testing.T) {
	kubeConfigContent := `
apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: cluster.host
  name: cluster
contexts:
- context:
    cluster: cluster
    user: user
  name: ctx
current-context: ctx
kind: Config
preferences: {}
users:
- name: user
  user:
    token: "123"
`
	kcfgFilePath := createFile(t, "testKubeConfig", kubeConfigContent)
	defer os.Remove(kcfgFilePath)

	configFileContent := `
serviceProviders:
- type: GitHub
  clientId: "123"
  clientSecret: "42"
- type: Quay
  clientId: "456"
  clientSecret: "54"
`
	cfgFilePath := createFile(t, "config", configFileContent)
	defer os.Remove(cfgFilePath)

	cfg, err := LoadFrom(&CommonCliArgs{ConfigFile: cfgFilePath, BaseUrl: "blabol"})
	assert.NoError(t, err)

	assert.Equal(t, "blabol", cfg.BaseUrl)
	assert.Len(t, cfg.ServiceProviders, 2)
}

func TestBaseUrlIsTrimmed(t *testing.T) {
	configFileContent := `
serviceProviders:
- type: GitHub
  clientId: "123"
  clientSecret: "42"
`
	cfgFilePath := createFile(t, "config", configFileContent)
	defer os.Remove(cfgFilePath)

	cfg, err := LoadFrom(&CommonCliArgs{ConfigFile: cfgFilePath, BaseUrl: "blabol/"})
	assert.NoError(t, err)

	assert.Equal(t, "blabol", cfg.BaseUrl)
}

func createFile(t *testing.T, path string, content string) string {
	file, err := os.CreateTemp(os.TempDir(), path)
	assert.NoError(t, err)

	assert.NoError(t, ioutil.WriteFile(file.Name(), []byte(content), fs.ModeExclusive))

	filePath, err := filepath.Abs(file.Name())
	assert.NoError(t, err)

	return filePath
}
