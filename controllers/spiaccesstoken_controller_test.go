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

package controllers

import (
	"context"
	"testing"

	"github.com/kcp-dev/logicalcluster/v2"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	sconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateUploadUrl(t *testing.T) {
	r := &SPIAccessTokenReconciler{
		Configuration: &opconfig.OperatorConfiguration{
			SharedConfiguration: sconfig.SharedConfiguration{
				BaseUrl: "blabol",
			},
		},
	}

	at := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-atname",
		},
	}

	t.Run("kcp-env", func(t *testing.T) {
		ctx := logicalcluster.WithCluster(context.TODO(), logicalcluster.New("workspace"))
		url := r.createUploadUrl(ctx, at)
		assert.Contains(t, url, "blabol")
		assert.Contains(t, url, "workspace")
		assert.Contains(t, url, "test-namespace")
		assert.Contains(t, url, "test-atname")
	})

	t.Run("non-kcp-env", func(t *testing.T) {
		url := r.createUploadUrl(context.TODO(), at)
		assert.Contains(t, url, "blabol")
		assert.NotContains(t, url, "workspace")
		assert.Contains(t, url, "test-namespace")
		assert.Contains(t, url, "test-atname")
	})
}
