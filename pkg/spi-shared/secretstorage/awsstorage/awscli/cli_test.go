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

package awscli

import (
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAwsSecretStorage(t *testing.T) {
	configFile := createFile(t, "awsconfig", "")
	defer os.Remove(configFile)
	credsFile := createFile(t, "awscreds", "")
	defer os.Remove(credsFile)

	storage, err := NewAwsSecretStorage(context.TODO(), &AWSCliArgs{ConfigFile: configFile, CredentialsFile: credsFile})

	assert.NoError(t, err)
	assert.NotNil(t, storage)
}

func TestNewAwsSecretStorageWithInvalidConfig(t *testing.T) {
	storage, err := NewAwsSecretStorage(context.TODO(), &AWSCliArgs{})

	assert.Error(t, err)
	assert.Nil(t, storage)
}

func TestValidateCliArgs(t *testing.T) {
	ctx := context.TODO()

	configFile := createFile(t, "awsconfig", "")
	defer os.Remove(configFile)
	credsFile := createFile(t, "awscreds", "")
	defer os.Remove(credsFile)

	assert.False(t, validateCliArgs(ctx, &AWSCliArgs{}))
	assert.False(t, validateCliArgs(ctx, &AWSCliArgs{ConfigFile: "blabol"}))
	assert.False(t, validateCliArgs(ctx, &AWSCliArgs{CredentialsFile: "blabol"}))
	assert.False(t, validateCliArgs(ctx, &AWSCliArgs{ConfigFile: "blbost", CredentialsFile: "blabol"}))

	assert.False(t, validateCliArgs(ctx, &AWSCliArgs{ConfigFile: configFile, CredentialsFile: "blabol"}))
	assert.False(t, validateCliArgs(ctx, &AWSCliArgs{ConfigFile: "blabol", CredentialsFile: credsFile}))

	assert.True(t, validateCliArgs(ctx, &AWSCliArgs{ConfigFile: configFile, CredentialsFile: credsFile}))
}

func TestCreateAwsConfig(t *testing.T) {
	ctx := context.TODO()

	configFile := createFile(t, "awsconfig", "")
	defer os.Remove(configFile)
	credsFile := createFile(t, "awscreds", "")
	defer os.Remove(credsFile)

	config, err := configFromCliArgs(ctx, &AWSCliArgs{ConfigFile: configFile, CredentialsFile: credsFile})

	assert.NoError(t, err)
	assert.NotNil(t, config)
}

func TestCreateAwsConfigWithInvalidArgs(t *testing.T) {
	config, err := configFromCliArgs(context.TODO(), &AWSCliArgs{})

	assert.Error(t, err)
	assert.Nil(t, config)
}

func createFile(t *testing.T, path string, content string) string {
	file, err := os.CreateTemp(os.TempDir(), path)
	assert.NoError(t, err)

	assert.NoError(t, ioutil.WriteFile(file.Name(), []byte(content), fs.ModeExclusive))

	filePath, err := filepath.Abs(file.Name())
	assert.NoError(t, err)

	return filePath
}
