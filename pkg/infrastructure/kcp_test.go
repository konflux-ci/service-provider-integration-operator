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

package infrastructure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kcp-dev/logicalcluster/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	apisv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestIsKcp(t *testing.T) {
	t.Run("is kcp", func(t *testing.T) {
		server := testApiServer(apiGroupsResponse(kcpApiGroup), http.StatusOK)
		defer server.Close()

		restConfig := &rest.Config{
			Host: server.URL,
		}

		isKcp, isKcpErr := IsKcp(restConfig)
		assert.NoError(t, isKcpErr)
		assert.True(t, isKcp)
	})

	t.Run("is not kcp", func(t *testing.T) {
		server := testApiServer(apiGroupsResponse([]metav1.APIGroup{}), http.StatusOK)
		defer server.Close()

		restConfig := &rest.Config{
			Host: server.URL,
		}

		isKcp, isKcpErr := IsKcp(restConfig)
		assert.NoError(t, isKcpErr)
		assert.False(t, isKcp)
	})

	t.Run("fail", func(t *testing.T) {
		server := testApiServer(apiGroupsResponse([]metav1.APIGroup{}), http.StatusInternalServerError)
		defer server.Close()

		restConfig := &rest.Config{
			Host: server.URL,
		}

		isKcp, isKcpErr := IsKcp(restConfig)
		assert.Error(t, isKcpErr)
		assert.False(t, isKcp)
	})
}

func TestRestApiConfig(t *testing.T) {
	t.Run("no apiexport name", func(t *testing.T) {
		newRestConfig, err := restConfigForAPIExport(context.TODO(), &rest.Config{}, "")
		assert.Nil(t, newRestConfig)
		assert.ErrorIs(t, err, missingApiExportNameError)
	})

	t.Run("no apiexport found", func(t *testing.T) {
		apiServer := testApiServer([]byte{}, http.StatusNotFound)
		defer apiServer.Close()

		restConfig := &rest.Config{
			Host: apiServer.URL,
		}

		newRestConfig, err := restConfigForAPIExport(context.TODO(), restConfig, "spi")

		assert.Error(t, err)
		assert.Nil(t, newRestConfig)
	})
}

func TestInitKcpControllerContext(t *testing.T) {
	t.Run("no workspace", func(t *testing.T) {
		ctx := InitKcpControllerContext(context.TODO(), ctrl.Request{})

		_, hasClusterName := logicalcluster.ClusterFromContext(ctx)
		assert.False(t, hasClusterName)
	})

	t.Run("kcp workspace", func(t *testing.T) {
		ctx := InitKcpControllerContext(context.TODO(), ctrl.Request{
			ClusterName: "workspace",
		})

		clusterName, hasClusterName := logicalcluster.ClusterFromContext(ctx)
		assert.True(t, hasClusterName)
		assert.Equal(t, "workspace", clusterName.String())
	})
}

var kcpApiGroup = []metav1.APIGroup{
	{
		Name: apisv1alpha1.SchemeGroupVersion.Group,
		Versions: []metav1.GroupVersionForDiscovery{
			{Version: apisv1alpha1.SchemeGroupVersion.Version},
		},
	},
}

func apiGroupsResponse(apiGroups []metav1.APIGroup) []byte {
	output, _ := json.Marshal(&metav1.APIGroupList{
		Groups: apiGroups,
	})

	return output
}

func testApiServer(response []byte, returnStatusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(returnStatusCode)
		w.Write(response)
	}))
}
