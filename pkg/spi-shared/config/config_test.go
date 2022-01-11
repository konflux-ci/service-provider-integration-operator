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
	"strings"
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
	os.Setenv(sharedSecretEnv, "secretValue")
	os.Setenv(baseUrlEnv, "baseUrlValue")

	kcfgFile, err := os.CreateTemp(os.TempDir(), "testKubeConfig")
	assert.NoError(t, err)
	defer os.Remove(kcfgFile.Name())

	assert.NoError(t, ioutil.WriteFile(kcfgFile.Name(), []byte(kubeConfigContent), fs.ModeExclusive))

	kcfgFilePath, err := filepath.Abs(kcfgFile.Name())
	assert.NoError(t, err)

	configFileContent := `
kubeConfigPath: ` + kcfgFilePath + `
serviceProviders:
- type: GitHub
  clientId: "123"
  clientSecret: "42"
- type: Quay
  clientId: "456"
  clientSecret: "54"
`

	pcfg, err := ReadFrom(strings.NewReader(configFileContent))
	assert.NoError(t, err)

	cfg, err := pcfg.Inflate()
	assert.NoError(t, err)

	assert.Equal(t, "baseUrlValue", cfg.BaseUrl)
	assert.Equal(t, []uint8("secretValue"), cfg.SharedSecret)

	assert.Equal(t, "cluster.host", cfg.KubernetesClientConfiguration.Host)
}
