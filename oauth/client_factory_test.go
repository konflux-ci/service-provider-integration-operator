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

package oauth

import (
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// minimal kubeconfig for testing
const kubeconfigContent = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://testkube
  name: testkube
contexts:
- context:
    cluster: testkube
    user: testkube
  name: testkube
current-context: testkube
`

func TestCustomizeKubeconfig(t *testing.T) {
	t.Run("auth provider is set", func(t *testing.T) {
		inKubeconfig := &rest.Config{}
		factoryConfig := &ClientFactoryConfig{}

		outKubeconfig, err := customizeKubeconfig(inKubeconfig, factoryConfig, true)

		assert.NotNil(t, outKubeconfig)
		assert.NotNil(t, outKubeconfig.AuthProvider)
		assert.NoError(t, err)
	})

	t.Run("auth provider is not set", func(t *testing.T) {
		inKubeconfig := &rest.Config{}
		factoryConfig := &ClientFactoryConfig{}

		outKubeconfig, err := customizeKubeconfig(inKubeconfig, factoryConfig, false)

		assert.NotNil(t, outKubeconfig)
		assert.Nil(t, outKubeconfig.AuthProvider)
		assert.NoError(t, err)
	})

	t.Run("insecure tls", func(t *testing.T) {
		inKubeconfig := &rest.Config{}
		factoryConfig := &ClientFactoryConfig{KubeInsecureTLS: true}

		outKubeconfig, err := customizeKubeconfig(inKubeconfig, factoryConfig, false)

		assert.NotNil(t, outKubeconfig)
		assert.True(t, outKubeconfig.Insecure)
		assert.NoError(t, err)
	})
}

func TestClientOptions(t *testing.T) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessToken"), meta.RESTScopeNamespace)

	opts, err := clientOptions(mapper)

	assert.NotNil(t, opts)
	assert.NoError(t, err)
	assert.NotNil(t, opts.Mapper)
	assert.NotNil(t, opts.Scheme)
}

func TestBaseKubernetesConfig(t *testing.T) {
	t.Run("set apiserver", func(t *testing.T) {
		config, err := baseKubernetesConfig(&ClientFactoryConfig{
			ApiServer: "https://testapiserver:1234",
		}, true)

		assert.NotNil(t, config)
		assert.Equal(t, "https://testapiserver:1234", config.Host)
		assert.NoError(t, err)
	})

	t.Run("apiserver not used when userAuthentication=false", func(t *testing.T) {
		// if for some reason this is running in pod, we temporarly unset KUBERNETES_SERVICE_HOST env so in-cluster configuration can fail
		// we want it to fail because we're testing that with some given cli args we try to create in-cluster config. not that it's actually created
		// it's easier to unset one env variable then create everything what is needed for in-cluster config (exact env vars and cert files at predefined paths)
		kubeHostBackup := os.Getenv("KUBERNETES_SERVICE_HOST")
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
		defer os.Setenv("KUBERNETES_SERVICE_HOST", kubeHostBackup)

		config, err := baseKubernetesConfig(&ClientFactoryConfig{
			ApiServer: "https://testapiserver:1234",
		}, false)

		assert.Nil(t, config)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to initialize in-cluster config")
	})

	t.Run("set apiserver with certpath", func(t *testing.T) {
		caFile := createFile(t, "cafile", "brokencert")
		defer os.Remove(caFile)

		config, err := baseKubernetesConfig(&ClientFactoryConfig{
			ApiServer:       "https://testapiserver:1234",
			ApiServerCAPath: caFile,
		}, true)

		assert.Nil(t, config)
		assert.Error(t, err)
		// we're testing that given cli arg tries to use given cert file. it fails because cert itself is invalid.
		assert.ErrorContains(t, err, "expected to load root CA config")
	})

	t.Run("set apiserver without port", func(t *testing.T) {
		config, err := baseKubernetesConfig(&ClientFactoryConfig{
			ApiServer: "https://testapiserver",
		}, true)

		assert.NotNil(t, config)
		assert.Equal(t, "https://testapiserver", config.Host)
		assert.NoError(t, err)
	})

	t.Run("kubeconfig is set over apiserver", func(t *testing.T) {
		kubeconfigPath := createFile(t, "kubeconfig", kubeconfigContent)
		defer os.Remove(kubeconfigPath)

		args := &ClientFactoryConfig{
			KubeConfig: kubeconfigPath,
			ApiServer:  "https://testapiserver",
		}
		config, err := baseKubernetesConfig(args, true)

		assert.NotNil(t, config)
		assert.Equal(t, "https://testkube", config.Host)
		assert.NoError(t, err)
	})

	t.Run("ok kubeconfig", func(t *testing.T) {
		kubeconfigPath := createFile(t, "kubeconfig", kubeconfigContent)
		defer os.Remove(kubeconfigPath)

		args := &ClientFactoryConfig{
			KubeConfig: kubeconfigPath,
		}
		config, err := baseKubernetesConfig(args, true)

		assert.NotNil(t, config)
		assert.Equal(t, "https://testkube", config.Host)
		assert.NoError(t, err)
	})

	t.Run("invlid kubeconfig", func(t *testing.T) {
		kubeconfigPath := createFile(t, "kubeconfig", "eh")
		defer os.Remove(kubeconfigPath)

		args := &ClientFactoryConfig{
			KubeConfig: kubeconfigPath,
		}
		config, err := baseKubernetesConfig(args, true)

		assert.Nil(t, config)
		assert.Error(t, err)
	})

	t.Run("unknown kubeconfig file", func(t *testing.T) {
		args := &ClientFactoryConfig{
			KubeConfig: "/blblblbl",
		}
		config, err := baseKubernetesConfig(args, true)

		assert.Nil(t, config)
		assert.Error(t, err)
	})

	t.Run("incluster kubeconfig", func(t *testing.T) {
		args := &ClientFactoryConfig{}

		// if for some reason this is running in pod, we temporarly unset KUBERNETES_SERVICE_HOST env so in-cluster configuration can fail
		// we want it to fail because we're testing that with some given cli args we try to create in-cluster config. not that it's actually created
		// it's easier to unset one env variable then create everything what is needed for in-cluster config (exact env vars and cert files at predefined paths)
		kubeHostBackup := os.Getenv("KUBERNETES_SERVICE_HOST")
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
		defer os.Setenv("KUBERNETES_SERVICE_HOST", kubeHostBackup)

		config, err := baseKubernetesConfig(args, true)

		assert.Nil(t, config)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to initialize in-cluster config")
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
