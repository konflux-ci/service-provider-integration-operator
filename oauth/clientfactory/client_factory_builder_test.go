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

package clientfactory

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/cmd/oauth/oauthcli"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestClientOptions(t *testing.T) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(v1beta1.GroupVersion.WithKind("SPIAccessToken"), meta.RESTScopeNamespace)

	opts, err := clientOptions(mapper)

	assert.NotNil(t, opts)
	assert.NoError(t, err)
	assert.NotNil(t, opts.Mapper)
	assert.NotNil(t, opts.Scheme)
}

func TestCustomizeRestconfig(t *testing.T) {
	t.Run("auth provider is set for user factories", func(t *testing.T) {

		cliArgs := oauthcli.OAuthServiceCliArgs{ApiServer: "https://testapiserver:1234"}
		builder := K8sClientFactoryBuilder{Args: cliArgs}

		clientFactory, err := builder.CreateUserAuthClientFactory()
		assert.NoError(t, err)

		userClientFactory, ok := clientFactory.(WorkspaceAwareK8sClientFactory)
		assert.True(t, ok)

		assert.NotNil(t, userClientFactory.RestConfig)
		assert.NotNil(t, userClientFactory.RestConfig.AuthProvider)

	})

	t.Run("auth provider is not set for in-cluster factory", func(t *testing.T) {
		kubeconfigPath := createFile(t, "kubeconfig", kubeconfigContent)
		defer os.Remove(kubeconfigPath)

		cliArgs := oauthcli.OAuthServiceCliArgs{KubeConfig: kubeconfigPath}
		builder := K8sClientFactoryBuilder{Args: cliArgs}

		clientFactory, err := builder.CreateInClusterClientFactory()
		assert.NoError(t, err)

		inClusterClientFactory, ok := clientFactory.(InClusterK8sClientFactory)
		assert.True(t, ok)

		assert.NotNil(t, inClusterClientFactory.RestConfig)
		assert.Nil(t, inClusterClientFactory.RestConfig.AuthProvider)

	})

	t.Run("insecure tls", func(t *testing.T) {
		kubeconfigPath := createFile(t, "kubeconfig", kubeconfigContent)
		defer os.Remove(kubeconfigPath)

		cliArgs := oauthcli.OAuthServiceCliArgs{KubeConfig: kubeconfigPath, KubeInsecureTLS: true}
		builder := K8sClientFactoryBuilder{Args: cliArgs}

		clientFactory, err := builder.CreateInClusterClientFactory()
		assert.NoError(t, err)

		inClusterClientFactory, ok := clientFactory.(InClusterK8sClientFactory)
		assert.True(t, ok)

		assert.NotNil(t, inClusterClientFactory.RestConfig)
		assert.True(t, inClusterClientFactory.RestConfig.Insecure)

	})

	t.Run("set apiserver for factory config", func(t *testing.T) {
		cliArgs := oauthcli.OAuthServiceCliArgs{ApiServer: "https://testapiserver:1234"}
		builder := K8sClientFactoryBuilder{Args: cliArgs}

		clientFactory, err := builder.CreateUserAuthClientFactory()
		assert.NoError(t, err)

		userClientFactory, ok := clientFactory.(WorkspaceAwareK8sClientFactory)
		assert.True(t, ok)

		assert.NotNil(t, userClientFactory.RestConfig)
		assert.NotNil(t, userClientFactory.RestConfig.Host)
	})

	t.Run("not set apiserver for in-cluster factory", func(t *testing.T) {
		// if for some reason this is running in pod, we temporarly unset KUBERNETES_SERVICE_HOST env so in-cluster configuration can fail
		// we want it to fail because we're testing that with some given cli args we try to create in-cluster config. not that it's actually created
		// it's easier to unset one env variable then create everything what is needed for in-cluster config (exact env vars and cert files at predefined paths)
		kubeHostBackup := os.Getenv("KUBERNETES_SERVICE_HOST")
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
		defer os.Setenv("KUBERNETES_SERVICE_HOST", kubeHostBackup)

		cliArgs := oauthcli.OAuthServiceCliArgs{ApiServer: "https://testapiserver:1234"}
		builder := K8sClientFactoryBuilder{Args: cliArgs}

		clientFactory, err := builder.CreateInClusterClientFactory()
		assert.Nil(t, clientFactory)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to initialize in-cluster config")

	})

	t.Run("set apiserver with certpath", func(t *testing.T) {
		caFile := createFile(t, "cafile", "brokencert")
		defer os.Remove(caFile)

		cliArgs := oauthcli.OAuthServiceCliArgs{ApiServerCAPath: caFile, ApiServer: "https://testapiserver:1234"}
		builder := K8sClientFactoryBuilder{Args: cliArgs}

		clientFactory, err := builder.CreateUserAuthClientFactory()

		assert.Nil(t, clientFactory)
		assert.Error(t, err)
		// we're testing that given cli arg tries to use given cert file. it fails because cert itself is invalid.
		assert.ErrorContains(t, err, "expected to load root CA config")
	})

	t.Run("set apiserver without port", func(t *testing.T) {
		cliArgs := oauthcli.OAuthServiceCliArgs{ApiServer: "https://testapiserver"}
		builder := K8sClientFactoryBuilder{Args: cliArgs}

		clientFactory, err := builder.CreateUserAuthClientFactory()
		assert.NoError(t, err)

		userClientFactory, ok := clientFactory.(WorkspaceAwareK8sClientFactory)
		assert.True(t, ok)

		assert.NotNil(t, userClientFactory.RestConfig)
		assert.NotNil(t, userClientFactory.RestConfig.Host)
		assert.Equal(t, "https://testapiserver", userClientFactory.RestConfig.Host)
	})

	t.Run("ok kubeconfig", func(t *testing.T) {
		kubeconfigPath := createFile(t, "kubeconfig", kubeconfigContent)
		defer os.Remove(kubeconfigPath)

		cliArgs := oauthcli.OAuthServiceCliArgs{KubeConfig: kubeconfigPath}
		builder := K8sClientFactoryBuilder{Args: cliArgs}

		clientFactory, err := builder.CreateInClusterClientFactory()
		assert.NoError(t, err)

		inClusterClientFactory, ok := clientFactory.(InClusterK8sClientFactory)
		assert.True(t, ok)

		assert.NotNil(t, inClusterClientFactory.RestConfig)
		assert.Equal(t, "https://testkube", inClusterClientFactory.RestConfig.Host)
	})

	t.Run("invalid kubeconfig", func(t *testing.T) {
		kubeconfigPath := createFile(t, "kubeconfig", "eh")
		defer os.Remove(kubeconfigPath)

		cliArgs := oauthcli.OAuthServiceCliArgs{KubeConfig: kubeconfigPath}
		builder := K8sClientFactoryBuilder{Args: cliArgs}

		clientFactory, err := builder.CreateInClusterClientFactory()
		assert.Error(t, err)
		assert.Nil(t, clientFactory)
	})

	t.Run("unknown kubeconfig file", func(t *testing.T) {
		cliArgs := oauthcli.OAuthServiceCliArgs{KubeConfig: "foo"}
		builder := K8sClientFactoryBuilder{Args: cliArgs}

		clientFactory, err := builder.CreateInClusterClientFactory()
		assert.Error(t, err)
		assert.Nil(t, clientFactory)
	})

	t.Run("incluster kubeconfig", func(t *testing.T) {
		// if for some reason this is running in pod, we temporarly unset KUBERNETES_SERVICE_HOST env so in-cluster configuration can fail
		// we want it to fail because we're testing that with some given cli args we try to create in-cluster config. not that it's actually created
		// it's easier to unset one env variable then create everything what is needed for in-cluster config (exact env vars and cert files at predefined paths)
		kubeHostBackup := os.Getenv("KUBERNETES_SERVICE_HOST")
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
		defer os.Setenv("KUBERNETES_SERVICE_HOST", kubeHostBackup)

		cliArgs := oauthcli.OAuthServiceCliArgs{}
		builder := K8sClientFactoryBuilder{Args: cliArgs}

		clientFactory, err := builder.CreateInClusterClientFactory()
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to initialize in-cluster config")
		assert.Nil(t, clientFactory)
	})
}

func createFile(t *testing.T, path string, content string) string {
	file, err := os.CreateTemp(os.TempDir(), path)
	assert.NoError(t, err)

	assert.NoError(t, os.WriteFile(file.Name(), []byte(content), fs.ModeExclusive))

	filePath, err := filepath.Abs(file.Name())
	assert.NoError(t, err)

	return filePath
}
