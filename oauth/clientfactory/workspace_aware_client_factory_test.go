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
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const wsResponseMock = `
{ 
 "kind": "WorkspaceList", 
 "apiVersion": "toolchain.dev.openshift.com/v1alpha1", 
 "metadata": {}, 
 "items": [ 
   { 
     "kind": "Workspace", 
     "metadata": { 
       "name": "foo" 
     }, 
     "status": { 
       "namespaces": [ 
         { 
           "name": "foo-tenant", 
           "type": "default" 
         } 
       ] 
     } 
   }, 
   { 
     "kind": "Workspace", 
     "metadata": { 
       "name": "bar" 
     }, 
     "status": { 
       "namespaces": [ 
         { 
           "name": "bar-tenant", 
           "type": "default" 
         } 
       ]
     } 
   }
  ]
}
`

const apiResponseMock = `
{
  "kind": "APIVersions",
  "versions": [],
  "serverAddressByClientCIDRs": [
    {
      "clientCIDR": "0.0.0.0/0",
      "serverAddress": "https://testapiserver:1234"
    }
  ]
}
`

var (
	apiserver = "https://testapiserver:1234"
)

func TestWorkspaceAwareK8sClientFactory(t *testing.T) {
	t.Run("workspace is set for workspace-aware user factories", func(t *testing.T) {
		clientFactory := WorkspaceAwareK8sClientFactory{
			RestConfig: &rest.Config{
				Host: apiserver,
				Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
					// making sure URL's path is set right. Otherwise, 500 is returned and client creation throws an error
					if r.URL.Path == "/workspaces/foo/api" || r.URL.Path == "/workspaces/foo/apis" {
						return &http.Response{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBuffer([]byte(apiResponseMock))),
						}, nil
					} else {
						return &http.Response{
							StatusCode: 500,
						}, nil
					}
				}),
			},
			ClientOptions: &client.Options{},
			ApiServer:     apiserver,
			HTTPClient: &http.Client{
				Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
					if r.URL.String() == apiserver+wsApiPath {
						return &http.Response{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBuffer([]byte(wsResponseMock))),
						}, nil
					} else {
						return &http.Response{
							StatusCode: 404,
						}, nil
					}
				}),
			}}

		namespacedContext := NamespaceIntoContext(context.TODO(), "foo-tenant")
		_, err := clientFactory.CreateClient(namespacedContext)
		assert.NoError(t, err)
	})

	t.Run("error thrown when workspace is not found", func(t *testing.T) {
		clientFactory := WorkspaceAwareK8sClientFactory{
			RestConfig: &rest.Config{
				Host: apiserver,
			},
			ClientOptions: &client.Options{},
			ApiServer:     apiserver,
			HTTPClient: &http.Client{
				Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
					if r.URL.String() == apiserver+wsApiPath {
						return &http.Response{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBuffer([]byte(wsResponseMock))),
						}, nil
					} else {
						return &http.Response{
							StatusCode: 404,
						}, nil
					}
				}),
			}}

		namespacedContext := NamespaceIntoContext(context.TODO(), "wrong")
		_, err := clientFactory.CreateClient(namespacedContext)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "target workspace not found for namespace")
	})

	t.Run("error thrown when workspace APi status is bad", func(t *testing.T) {
		clientFactory := WorkspaceAwareK8sClientFactory{
			RestConfig: &rest.Config{
				Host: apiserver,
			},
			ClientOptions: &client.Options{},
			ApiServer:     apiserver,
			HTTPClient: &http.Client{
				Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 404,
					}, nil
				}),
			}}

		namespacedContext := NamespaceIntoContext(context.TODO(), "wrong")
		_, err := clientFactory.CreateClient(namespacedContext)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "bad status (404) when performing HTTP request for workspace")
	})

	t.Run("return simple client when no namespace set into context", func(t *testing.T) {
		clientFactory := WorkspaceAwareK8sClientFactory{
			RestConfig: &rest.Config{
				Host: apiserver,
				Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
					// making sure URL's path is default. Otherwise, 500 is returned and client creation throws an error
					if r.URL.Path == "/api" || r.URL.Path == "/apis" {
						return &http.Response{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewBuffer([]byte(apiResponseMock))),
						}, nil
					} else {
						return &http.Response{
							StatusCode: 500,
						}, nil
					}
				}),
			},
			ClientOptions: &client.Options{},
			ApiServer:     apiserver,
			HTTPClient:    &http.Client{},
		}
		_, err := clientFactory.CreateClient(context.TODO())
		assert.NoError(t, err)
	})
}
