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
	"bytes"
	"context"

	"github.com/google/go-github/v45/github"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"golang.org/x/oauth2"

	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
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
	matchesFunc func(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error)
}

func (t tokenFilterMock) Matches(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
	return t.matchesFunc(ctx, matchable, token)
}

func TestMain(m *testing.M) {
	logs.InitDevelLoggers()
	os.Exit(m.Run())
}
func TestCheckPublicRepo(t *testing.T) {
	test := func(statusCode int, expected bool) {
		t.Run(fmt.Sprintf("code %d => %t", statusCode, expected), func(t *testing.T) {
			gh := Github{httpClient: httpClientMock{
				doFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: statusCode, Body: io.NopCloser(strings.NewReader(""))}, nil
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
	gh := mockGithub(cl, http.StatusOK, nil, nil)

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
	gh := mockGithub(cl, http.StatusServiceUnavailable, fmt.Errorf("fail to talk to github api"), nil)

	ac := api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: testValidRepoUrl},
	}

	status, err := gh.CheckRepositoryAccess(context.TODO(), cl, &ac)

	assert.Error(t, err)
	assert.Nil(t, status)
}

func TestCheckAccessPrivate(t *testing.T) {
	cl := mockK8sClient()
	gh := mockGithub(cl, http.StatusNotFound, nil, nil)
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
	gh := mockGithub(cl, http.StatusNotFound, nil, nil)
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

func TestCheckAccessFailingLookupPublicRepo(t *testing.T) {
	cl := mockK8sClient(&api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "token",
			Namespace: "ac-namespace",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: string(api.ServiceProviderTypeGitHub),
				api.ServiceProviderHostLabel: config.ServiceProviderTypeGitHub.DefaultHost,
			},
		},
		Spec: api.SPIAccessTokenSpec{
			ServiceProviderUrl: config.ServiceProviderTypeGitHub.DefaultBaseUrl,
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
			TokenMetadata: &api.TokenMetadata{
				LastRefreshTime: time.Now().Add(time.Hour).Unix(),
			},
		},
	})
	gh := mockGithub(cl, http.StatusOK, nil, errors.New("intentional failure"))

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
	assert.Empty(t, status.ErrorReason)
	assert.Empty(t, status.ErrorMessage)
}

func TestCheckAccessFailingLookupNonPublicRepo(t *testing.T) {
	cl := mockK8sClient(&api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "token",
			Namespace: "ac-namespace",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: string(api.ServiceProviderTypeGitHub),
				api.ServiceProviderHostLabel: config.ServiceProviderTypeGitHub.DefaultHost,
			},
		},
		Spec: api.SPIAccessTokenSpec{
			ServiceProviderUrl: config.ServiceProviderTypeGitHub.DefaultBaseUrl,
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
			TokenMetadata: &api.TokenMetadata{
				LastRefreshTime: time.Now().Add(time.Hour).Unix(),
			},
		},
	})
	gh := mockGithub(cl, http.StatusNotFound, nil, errors.New("intentional failure"))

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
	assert.Equal(t, api.SPIAccessCheckErrorTokenLookupFailed, status.ErrorReason)
	assert.NotEmpty(t, status.ErrorMessage)
}

func TestCheckAccessWithMatchingTokens(t *testing.T) {
	cl := mockK8sClient(&api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "token",
			Namespace: "ac-namespace",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: string(api.ServiceProviderTypeGitHub),
				api.ServiceProviderHostLabel: config.ServiceProviderTypeGitHub.DefaultHost,
			},
		},
		Spec: api.SPIAccessTokenSpec{
			ServiceProviderUrl: config.ServiceProviderTypeGitHub.DefaultBaseUrl,
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
			TokenMetadata: &api.TokenMetadata{
				LastRefreshTime: time.Now().Add(time.Hour).Unix(),
			},
		},
	})
	gh := mockGithub(cl, http.StatusOK, nil, nil)
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

func TestValidate(t *testing.T) {
	g := &Github{}

	res, err := g.Validate(context.TODO(), &api.SPIAccessToken{
		Spec: api.SPIAccessTokenSpec{
			Permissions: api.Permissions{
				AdditionalScopes: []string{"blah"},
			},
		},
	})
	assert.NoError(t, err)

	assert.Equal(t, 1, len(res.ScopeValidation))
	assert.NotNil(t, res.ScopeValidation[0])
	assert.Equal(t, "unknown scope: 'blah'", res.ScopeValidation[0].Error())
}

func TestNewGithubOauthCapability(t *testing.T) {
	t.Run("nil when nil config", func(t *testing.T) {
		oauthCapability := newGithubOAuthCapability(&serviceprovider.Factory{}, nil)
		assert.Nil(t, oauthCapability)
	})
	t.Run("nil when empty config", func(t *testing.T) {
		oauthCapability := newGithubOAuthCapability(&serviceprovider.Factory{}, &config.ServiceProviderConfiguration{})
		assert.Nil(t, oauthCapability)
	})
	t.Run("created when config ok", func(t *testing.T) {
		oauthCapability := newGithubOAuthCapability(
			&serviceprovider.Factory{
				Configuration: &opconfig.OperatorConfiguration{
					SharedConfiguration: config.SharedConfiguration{
						BaseUrl: "base.url",
					},
				},
			},
			&config.ServiceProviderConfiguration{
				OAuth2Config: &oauth2.Config{},
			})
		assert.NotNil(t, oauthCapability)
		assert.Contains(t, oauthCapability.GetOAuthEndpoint(), "base.url")
	})
}

func TestNewGithub(t *testing.T) {
	factory := &serviceprovider.Factory{
		Configuration: &opconfig.OperatorConfiguration{
			TokenLookupCacheTtl: 1 * time.Minute,
		},
		HttpClient: http.DefaultClient,
	}
	spConfig := &config.ServiceProviderConfiguration{}

	sp, err := newGithub(factory, spConfig)

	assert.NoError(t, err)
	assert.NotNil(t, sp)
	assert.Nil(t, sp.GetOAuthCapability())
	assert.NotNil(t, sp.GetDownloadFileCapability())
	assert.Equal(t, config.ServiceProviderTypeGitHub, sp.GetType())
	assert.Equal(t, config.ServiceProviderTypeGitHub.DefaultBaseUrl, sp.GetBaseUrl())
}

func mockGithub(cl client.Client, returnCode int, httpErr error, lookupError error) *Github {
	metadataCache := serviceprovider.MetadataCache{
		Client:                    cl,
		ExpirationPolicy:          &serviceprovider.NeverMetadataExpirationPolicy{},
		CacheServiceProviderState: true,
	}
	ts := tokenstorage.TestTokenStorage{GetImpl: func(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
		return &api.Token{AccessToken: "blabol"}, nil
	}}

	mockedHTTPClient := &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: returnCode,
				Header:     http.Header{},
				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(`{"message": "error"}`))),
				Request:    r,
			}, httpErr
		}),
	}

	return &Github{httpClient: mockedHTTPClient,

		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeGitHub,
			MetadataCache:       &metadataCache,
			TokenFilter: tokenFilterMock{
				matchesFunc: func(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
					return true, lookupError
				},
			},
			RepoHostParser: serviceprovider.RepoHostFromUrl,
		},
		tokenStorage: ts,
		ghClientBuilder: githubClientBuilder{
			httpClient:   mockedHTTPClient,
			tokenStorage: ts,
		},
	}
}

func TestGitHubUnexpectedStatusMetric(t *testing.T) {
	// given
	unexpectedStatusCounter.Reset()
	numberOfResponses := 3
	expectedLabels := map[string]string{"unexpected_status": "429"}
	gh := mockGithub(nil, http.StatusTooManyRequests, nil, nil)
	for i := 0; i < numberOfResponses; i++ {
		public, err := gh.publicRepo(context.TODO(), &api.SPIAccessCheck{
			Spec: api.SPIAccessCheckSpec{RepoUrl: "test.com"},
		})
		assert.NoError(t, err)
		assert.False(t, public)
	}

	// when
	mfs, err := metrics.Registry.Gather()
	assert.NoError(t, err)

	// then
	metricPresent := false
	for _, mf := range mfs {
		if mf.GetName() == "redhat_appstudio_spi_github_public_repo_unexpected_response_status" {
			for _, m := range mf.GetMetric() {
				assert.Equal(t, float64(numberOfResponses), *m.Counter.Value)
				for _, lp := range m.Label {
					assert.Equal(t, expectedLabels[*lp.Name], *lp.Value)
				}
				metricPresent = true
			}
		}
	}
	assert.True(t, metricPresent)
}

func TestGitHubRateLimitErrorMetric(t *testing.T) {
	// given
	numberOfResponses := 3
	cl := mockK8sClient(&api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "token",
			Namespace: "ac-namespace",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: string(api.ServiceProviderTypeGitHub),
				api.ServiceProviderHostLabel: config.ServiceProviderTypeGitHub.DefaultHost,
			},
		},
		Spec: api.SPIAccessTokenSpec{
			ServiceProviderUrl: config.ServiceProviderTypeGitHub.DefaultBaseUrl,
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
			TokenMetadata: &api.TokenMetadata{
				LastRefreshTime: time.Now().Add(time.Hour).Unix(),
			},
		},
	})
	gh := mockGithub(cl, http.StatusNotFound, nil, nil)
	gh.ghClientBuilder.httpClient = &http.Client{
		Transport: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return nil, &github.RateLimitError{
				Rate: github.Rate{
					Limit:     5000,
					Remaining: 0,
					Reset:     github.Timestamp{},
				},
				Response: nil,
				Message:  "rate limit exceeded",
			}
		}),
	}
	ac := &api.SPIAccessCheck{
		Spec: api.SPIAccessCheckSpec{RepoUrl: "https://github.com/user/repo"},
	}
	for i := 0; i < numberOfResponses; i++ {
		accessCheckStatus, err := gh.CheckRepositoryAccess(context.TODO(), cl, ac)
		assert.NoError(t, err)
		assert.NotNil(t, accessCheckStatus)
	}

	// when
	mfs, err := metrics.Registry.Gather()
	assert.NoError(t, err)

	// then
	metricPresent := false
	for _, mf := range mfs {
		if mf.GetName() == "redhat_appstudio_spi_github_rate_limit_errors" {
			for _, m := range mf.GetMetric() {
				assert.Equal(t, float64(numberOfResponses), *m.Counter.Value)
				metricPresent = true
			}
		}
	}
	assert.True(t, metricPresent)
}

func mockK8sClient(objects ...client.Object) client.WithWatch {
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(objects...).Build()
}
