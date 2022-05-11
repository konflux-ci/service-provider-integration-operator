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

package quay

import (
	"context"
	"net/http"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testValidRepoUrl = "https://quay.io/repository/redhat-appstudio/service-provider-integration-operator"

func TestQuayProbe_Examine(t *testing.T) {
	probe := quayProbe{}
	test := func(t *testing.T, url string, expectedMatch bool) {
		baseUrl, err := probe.Examine(nil, url)
		expectedBaseUrl := ""
		if expectedMatch {
			expectedBaseUrl = "https://quay.io"
		}

		assert.NoError(t, err)
		assert.Equal(t, expectedBaseUrl, baseUrl)
	}

	test(t, "quay.io/name/repo", true)
	test(t, "https://quay.io/name/repo", true)
	test(t, "https://github.com/name/repo", false)
	test(t, "github.com/name/repo", false)
}

func TestCheckAccessNotImplementedYetError(t *testing.T) {
	cl := mockK8sClient()
	quay := mockQuay(cl, http.StatusNotFound, nil)
	ac := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: testValidRepoUrl},
	}

	status, err := quay.CheckRepositoryAccess(context.TODO(), cl, &ac)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, api.SPIAccessCheckErrorNotImplemented, status.ErrorReason)
}

type httpClientMock struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (h httpClientMock) Do(req *http.Request) (*http.Response, error) {
	return h.doFunc(req)
}

type tokenFilterMock struct {
	matchesFunc func(matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error)
}

func (t tokenFilterMock) Matches(matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
	return t.matchesFunc(matchable, token)
}

func mockQuay(cl client.Client, returnCode int, httpErr error) *Quay {
	metadataCache := serviceprovider.NewMetadataCache(0, cl)
	return &Quay{
		httpClient: httpClientMock{
			doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: returnCode}, httpErr
			},
		},
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeGitHub,
			MetadataCache:       &metadataCache,
			TokenFilter: tokenFilterMock{
				matchesFunc: func(matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
					return true, nil
				},
			},
		},
	}
}

func mockK8sClient(objects ...client.Object) client.WithWatch {
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(objects...).Build()
}
