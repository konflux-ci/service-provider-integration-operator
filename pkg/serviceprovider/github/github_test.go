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

package github

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

const testValidRepoUrl = "https://github.com/redhat-appstudio/service-provider-integration-operator"

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

type tokenStorageMock struct {
	getFunc func(ctx context.Context, owner *api.SPIAccessToken) *api.Token
}

func (t tokenStorageMock) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	return nil
}

func (t tokenStorageMock) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	return t.getFunc(ctx, owner), nil
}

func (t tokenStorageMock) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	return nil
}

func TestCheckPublicRepo(t *testing.T) {
	test := func(statusCode int, expected bool) {
		t.Run(fmt.Sprintf("code %d => %t", statusCode, expected), func(t *testing.T) {
			gh := Github{httpClient: httpClientMock{
				doFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: statusCode}, nil
				},
			}}
			spiAccessCheck := &api.SPIAccessCheck{Spec: api.SPIAccessCheckSpec{RepoUrl: "test"}}

			publicRepo, err := gh.publicRepo(context.TODO(), spiAccessCheck)

			assert.NoError(t, err)
			assert.Equal(t, expected, publicRepo)
		})
	}

	test(200, true)
	test(404, false)
	test(403, false)
	test(500, false)
	test(666, false)

	t.Run("fail", func(t *testing.T) {
		gh := Github{httpClient: httpClientMock{
			doFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("error")
			},
		}}
		spiAccessCheck := &api.SPIAccessCheck{Spec: api.SPIAccessCheckSpec{RepoUrl: "test"}}

		publicRepo, err := gh.publicRepo(context.TODO(), spiAccessCheck)

		assert.Error(t, err)
		assert.Equal(t, false, publicRepo)
	})
}

func TestParseGithubRepositoryUrl(t *testing.T) {
	gh := Github{}

	testOk := func(url, expectedOwner, expectedRepo string) {
		t.Run(fmt.Sprintf("%s => %s:%s", url, expectedOwner, expectedRepo), func(t *testing.T) {
			owner, repo, err := gh.parseGithubRepoUrl(url)
			assert.NoError(t, err)
			assert.Equal(t, expectedOwner, owner)
			assert.Equal(t, expectedRepo, repo)
		})
	}
	testFail := func(url string) {
		t.Run(fmt.Sprintf("%s fails", url), func(t *testing.T) {
			owner, repo, err := gh.parseGithubRepoUrl(url)
			assert.Error(t, err)
			assert.Empty(t, owner)
			assert.Empty(t, repo)
		})
	}

	testOk("https://github.com/redhat-appstudio/service-provider-integration-operator",
		"redhat-appstudio", "service-provider-integration-operator")
	testOk("https://github.com/redhat-appstudio/service-provider-integration-operator/something/else",
		"redhat-appstudio", "service-provider-integration-operator")
	testOk("https://github.com/sparkoo/service-provider-integration-operator",
		"sparkoo", "service-provider-integration-operator")

	testFail("https://blabol.com/redhat-appstudio/service-provider-integration-operator")
	testFail("")
	testFail("https://github.com/redhat-appstudio")
}

func TestCheckAccess(t *testing.T) {
	cl := mockK8sClient()
	gh := mockGithub(cl, http.StatusOK, nil)

	ac := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: testValidRepoUrl},
	}

	status, err := gh.CheckRepositoryAccess(context.TODO(), cl, &ac)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.True(t, status.Accessible)
	assert.Equal(t, api.SPIRepoTypeGit, status.Type)
	assert.Equal(t, api.ServiceProviderTypeGitHub, status.ServiceProvider)
	assert.Equal(t, api.SPIAccessCheckAccessibilityPublic, status.Accessibility)
}

func TestFailWithGithubHttp(t *testing.T) {
	cl := mockK8sClient()
	gh := mockGithub(cl, http.StatusServiceUnavailable, fmt.Errorf("fail to talk to github api"))

	ac := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: testValidRepoUrl},
	}

	status, err := gh.CheckRepositoryAccess(context.TODO(), cl, &ac)

	assert.Error(t, err)
	assert.Nil(t, status)
}

func TestCheckAccessPrivate(t *testing.T) {
	cl := mockK8sClient()
	gh := mockGithub(cl, http.StatusNotFound, nil)
	ac := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: testValidRepoUrl},
	}

	status, err := gh.CheckRepositoryAccess(context.TODO(), cl, &ac)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.False(t, status.Accessible)
	assert.Equal(t, api.SPIRepoTypeGit, status.Type)
	assert.Equal(t, api.ServiceProviderTypeGitHub, status.ServiceProvider)
	assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
}

func TestCheckAccessBadUrl(t *testing.T) {
	cl := mockK8sClient()
	gh := mockGithub(cl, http.StatusNotFound, nil)
	ac := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: "blabol.this.is.not.github.url"},
	}

	status, err := gh.CheckRepositoryAccess(context.TODO(), cl, &ac)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.False(t, status.Accessible)
	assert.Equal(t, api.SPIRepoTypeGit, status.Type)
	assert.Equal(t, api.ServiceProviderTypeGitHub, status.ServiceProvider)
	assert.Equal(t, api.SPIAccessCheckAccessibilityUnknown, status.Accessibility)
	assert.Equal(t, api.SPIAccessCheckErrorBadURL, status.ErrorReason)
}

func TestCheckAccessWithMatchingTokens(t *testing.T) {
	cl := mockK8sClient(&api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "token",
			Namespace: "ac-namespace",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: string(api.ServiceProviderTypeGitHub),
				api.ServiceProviderHostLabel: "github.com",
			},
		},
		Spec: api.SPIAccessTokenSpec{
			ServiceProviderUrl: "https://github.com",
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
			TokenMetadata: &api.TokenMetadata{
				LastRefreshTime: time.Now().Add(time.Hour).Unix(),
			},
		},
	})
	gh := mockGithub(cl, http.StatusOK, nil)
	ac := api.SPIAccessCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "access-check",
			Namespace: "ac-namespace",
		},
		Spec: api.SPIAccessCheckSpec{RepoUrl: testValidRepoUrl},
	}

	status, err := gh.CheckRepositoryAccess(context.TODO(), cl, &ac)

	assert.NoError(t, err)
	assert.NotNil(t, status)
}

func mockGithub(cl client.Client, returnCode int, httpErr error) *Github {
	metadataCache := serviceprovider.NewMetadataCache(0, cl)
	return &Github{
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
		tokenStorage: tokenStorageMock{getFunc: func(ctx context.Context, owner *api.SPIAccessToken) *api.Token {
			return &api.Token{AccessToken: "blabol"}
		}},
	}
}

func mockK8sClient(objects ...client.Object) client.WithWatch {
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(objects...).Build()
}
