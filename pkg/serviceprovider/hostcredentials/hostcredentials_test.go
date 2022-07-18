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

package hostcredentials

import (
	"context"
	"os"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMain(m *testing.M) {
	logs.InitDevelLoggers()
	os.Exit(m.Run())
}

func TestGetBaseUrl(t *testing.T) {
	repoURLs := map[string]string{"https://snyk.io/": "https://snyk.io", "https://testing.io/foo/bar?a=b": "https://testing.io"}
	for input, expected := range repoURLs {
		common := &HostCredentialsProvider{repoUrl: input}
		assert.Equal(t, expected, common.GetBaseUrl())
	}
}

func TestMapToken(t *testing.T) {
	common := &HostCredentialsProvider{}
	mapper, err := common.MapToken(context.TODO(), &api.SPIAccessTokenBinding{},
		&api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: "spiat1",
			},
			Status: api.SPIAccessTokenStatus{
				TokenMetadata: &api.TokenMetadata{
					Username:             "alois",
					UserId:               "42",
					Scopes:               nil,
					ServiceProviderState: []byte(""),
				},
			},
		}, &api.Token{
			AccessToken: "access_token",
		})
	assert.NoError(t, err)
	assert.Equal(t, "spiat1", mapper.Name)
	assert.Equal(t, "alois", mapper.ServiceProviderUserName)
	assert.Equal(t, "42", mapper.ServiceProviderUserId)
	assert.Equal(t, "access_token", mapper.Token)
}
