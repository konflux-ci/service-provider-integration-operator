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
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/stretchr/testify/assert"
)

func TestRead(t *testing.T) {
	SetupCustomValidations(CustomValidationOptions{AllowInsecureURLs: true})
	t.Run("all supported service providers are set", func(t *testing.T) {
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

		cfg, err := LoadFrom(cfgFilePath, "blabol")
		assert.NoError(t, err)

		assert.Equal(t, "blabol", cfg.BaseUrl)
		assert.Len(t, cfg.ServiceProviders, len(SupportedServiceProviderTypes))
	})

	t.Run("all supported service providers are set even if config is empty", func(t *testing.T) {
		configFileContent := `
`
		cfgFilePath := createFile(t, "config", configFileContent)
		defer os.Remove(cfgFilePath)

		cfg, err := LoadFrom(cfgFilePath, "blabol")
		assert.NoError(t, err)

		assert.Equal(t, "blabol", cfg.BaseUrl)
		assert.Len(t, cfg.ServiceProviders, len(SupportedServiceProviderTypes))
	})

	t.Run("unknown service provider result in error", func(t *testing.T) {
		configFileContent := `
serviceProviders:
- type: blabol
  clientId: "123"
  clientSecret: "42"
`
		cfgFilePath := createFile(t, "config", configFileContent)
		defer os.Remove(cfgFilePath)

		cfg, err := LoadFrom(cfgFilePath, "blabol")
		assert.Error(t, err)
		assert.Empty(t, cfg.BaseUrl)
		assert.Empty(t, cfg.ServiceProviders)
	})

	t.Run("wrong content result in error", func(t *testing.T) {
		configFileContent := `blabol`
		cfgFilePath := createFile(t, "config", configFileContent)
		defer os.Remove(cfgFilePath)

		cfg, err := LoadFrom(cfgFilePath, "blabol")
		assert.Error(t, err)
		assert.Empty(t, cfg.BaseUrl)
		assert.Empty(t, cfg.ServiceProviders)
	})

	t.Run("file not exist", func(t *testing.T) {
		cfg, err := LoadFrom("blbost", "blabol")
		assert.Error(t, err)
		assert.Empty(t, cfg.BaseUrl)
		assert.Empty(t, cfg.ServiceProviders)
	})

	t.Run("read error", func(t *testing.T) {
		cfg, err := readFrom(&r{})

		assert.Error(t, err)
		assert.Empty(t, cfg.ServiceProviders)
	})
}

type r struct{}

func (r *r) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("some error")
}

func TestBaseUrlIsTrimmed(t *testing.T) {
	SetupCustomValidations(CustomValidationOptions{AllowInsecureURLs: true})
	configFileContent := `
serviceProviders:
- type: GitHub
  clientId: "123"
  clientSecret: "42"
`
	cfgFilePath := createFile(t, "config", configFileContent)
	defer os.Remove(cfgFilePath)

	cfg, err := LoadFrom(cfgFilePath, "blabol/")
	assert.NoError(t, err)

	assert.Equal(t, "blabol", cfg.BaseUrl)
}

func TestBaseUrlIsValidated(t *testing.T) {
	SetupCustomValidations(CustomValidationOptions{AllowInsecureURLs: false})
	t.Run("config is validated", func(t *testing.T) {

		configFileContent := `
serviceProviders:
- type: GitHub
  clientId: "123"
  clientSecret: "42"
`
		cfgFilePath := createFile(t, "config", configFileContent)
		defer os.Remove(cfgFilePath)

		_, err := LoadFrom(cfgFilePath, "blabol/")
		var validationErr validator.ValidationErrors
		assert.True(t, errors.As(err, &validationErr))
		assert.NotNil(t, validationErr.Error())
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
