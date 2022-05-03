package github

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

type httpClientMock struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (h httpClientMock) Do(req *http.Request) (*http.Response, error) {
	return h.doFunc(req)
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

			publicRepo := gh.publicRepo(context.TODO(), spiAccessCheck)

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

		publicRepo := gh.publicRepo(context.TODO(), spiAccessCheck)

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
