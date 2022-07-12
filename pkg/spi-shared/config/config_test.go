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
sharedSecret: yaddayadda123$@#**
serviceProviders:
- type: GitHub
  clientId: "123"
  clientSecret: "42"
- type: Quay
  clientId: "456"
  clientSecret: "54"
baseUrl: blabol
vault:
  host: vaultTestHost
  kubernetes:
    serviceAccountTokenFilePath: "/over/the/rainbow"
accessCheckTtl: 37m
tokenLookupCacheTtl: 62m
`
	cfgFilePath := createFile(t, "config", configFileContent)
	defer os.Remove(cfgFilePath)

	cfg, err := LoadFrom(cfgFilePath)
	assert.NoError(t, err)

	assert.Equal(t, "blabol", cfg.BaseUrl)
	assert.Equal(t, []byte("yaddayadda123$@#**"), cfg.SharedSecret)
	assert.Equal(t, "vaultTestHost", cfg.VaultConfiguration.Host)
	assert.Equal(t, "/over/the/rainbow", cfg.VaultConfiguration.KubernetesAuthentication.ServiceAccountTokenFilePath)
	assert.Equal(t, time.Minute*37, cfg.AccessCheckTtl)
	assert.Equal(t, time.Minute*62, cfg.TokenLookupCacheTtl)
	assert.Len(t, cfg.ServiceProviders, 2)
}

func TestDefaults(t *testing.T) {
	configFileContent := `
`
	cfgFilePath := createFile(t, "config", configFileContent)
	defer os.Remove(cfgFilePath)

	cfg, err := LoadFrom(cfgFilePath)
	assert.NoError(t, err)

	assert.Equal(t, "http://spi-vault:8200", cfg.VaultConfiguration.Host)
	assert.Equal(t, time.Minute*30, cfg.AccessCheckTtl)
	assert.Equal(t, time.Hour, cfg.TokenLookupCacheTtl)
}

func TestEnvOverrides(t *testing.T) {
	configFileContent := `
sharedSecret: yaddayadda123$@#**
serviceProviders:
- type: GitHub
  clientId: "123"
  clientSecret: "42"
- type: Quay
  clientId: "456"
  clientSecret: "54"
baseUrl: blabol
vault:
  host: vaultTestHost
  kubernetes:
    serviceAccountTokenFilePath: "/over/the/rainbow"
accessCheckTtl: 37m
tokenLookupCacheTtl: 62m
`
	cfgFilePath := createFile(t, "config", configFileContent)
	defer os.Remove(cfgFilePath)

	testWithEnv := func(name, value string, test func(*testing.T, Configuration)) {
		t.Run(name, func(t *testing.T) {
			origVal, existed := os.LookupEnv(name)
			assert.NoError(t, os.Setenv(name, value))

			cfg, err := LoadFrom(cfgFilePath)
			assert.NoError(t, err)

			test(t, cfg)

			if existed {
				assert.NoError(t, os.Setenv(name, origVal))
			} else {
				assert.NoError(t, os.Unsetenv(name))
			}
		})
	}

	testWithEnv("KUBERNETES_AUDIENCES", "a,b,c", func(t *testing.T, c Configuration) {
		assert.Equal(t, []string{"a", "b", "c"}, c.KubernetesAuthAudiences)
	})

	testWithEnv("JWT_SHARED_SECRET", "secret", func(t *testing.T, c Configuration) {
		assert.Equal(t, []byte("secret"), c.SharedSecret)
	})

	testWithEnv("BASE_URL", "https://base.url", func(t *testing.T, c Configuration) {
		assert.Equal(t, "https://base.url", c.BaseUrl)
	})

	testWithEnv("TOKEN_LOOKUP_CACHE_TTL", "2m", func(t *testing.T, c Configuration) {
		assert.Equal(t, 2*time.Minute, c.TokenLookupCacheTtl)
	})

	testWithEnv("ACCESS_CHECK_TTL", "3s", func(t *testing.T, c Configuration) {
		assert.Equal(t, 3*time.Second, c.AccessCheckTtl)
	})

	testWithEnv("VAULT_HOST", "https://vault.remote", func(t *testing.T, c Configuration) {
		assert.Equal(t, "https://vault.remote", c.VaultConfiguration.Host)
	})

	testWithEnv("SA_TOKEN_PATH", "/the/path", func(t *testing.T, c Configuration) {
		assert.Equal(t, "/the/path", c.VaultConfiguration.KubernetesAuthentication.ServiceAccountTokenFilePath)
	})
}

func TestTtlParseFail(t *testing.T) {
	test := func(configFileContent string) {

		cfgFilePath := createFile(t, "config", configFileContent)
		defer os.Remove(cfgFilePath)

		_, err := LoadFrom(cfgFilePath)
		assert.Error(t, err)
	}
	t.Run("accessCheckTtl", func(t *testing.T) {
		test("accessCheckTtl: blabol")
	})

	t.Run("tokenLookupCacheTtl", func(t *testing.T) {
		test("tokenLookupCacheTtl: blabol")
	})
}

func TestParseDuration(t *testing.T) {
	t.Run("fail orig", func(t *testing.T) {
		d, err := parseDuration("blabol", "1h")
		assert.Empty(t, d)
		assert.Error(t, err)
	})
	t.Run("fail default", func(t *testing.T) {
		d, err := parseDuration("", "blabol")
		assert.Empty(t, d)
		assert.Error(t, err)
	})
	t.Run("ok orig", func(t *testing.T) {
		d, err := parseDuration("1h23m", "1h")
		assert.Equal(t, 83*time.Minute, d)
		assert.NoError(t, err)
	})
	t.Run("ok default", func(t *testing.T) {
		d, err := parseDuration("", "1h23m")
		assert.Equal(t, 83*time.Minute, d)
		assert.NoError(t, err)
	})
	t.Run("empty", func(t *testing.T) {
		d, err := parseDuration("", "")
		assert.Empty(t, d)
		assert.Error(t, err)
	})
}

func createFile(t *testing.T, path string, content string) string {
	file, err := os.CreateTemp(os.TempDir(), path)
	assert.NoError(t, err)

	assert.NoError(t, ioutil.WriteFile(file.Name(), []byte(content), fs.ModeExclusive))

	filePath, err := filepath.Abs(file.Name())
	assert.NoError(t, err)

	return filePath
}
