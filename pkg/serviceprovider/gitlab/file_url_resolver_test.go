package gitlab

import (
	"context"
	"fmt"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestGetFileHead(t *testing.T) {
	gitlabReached := false

	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == "https://fake.github.com/api/v4/projects/foo-user%2Ffoo-repo/repository/files/myfile?ref=HEAD" {
				gitlabReached = true
				return &http.Response{
					StatusCode: 200,
					Header: http.Header{
						"content-type":  {"application/json"},
						"x-gitlab-ref":  {"main"},
						"x-gitlab-size": {"6192"},
					},
					Body:    ioutil.NopCloser(strings.NewReader("")),
					Request: r,
				}, nil
			}

			return nil, fmt.Errorf("unexpected request to: %s", r.URL.String())
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
			return &api.Token{
				AccessToken:  "access",
				TokenType:    "fake",
				RefreshToken: "refresh",
				Expiry:       0,
			}, nil
		},
	}
	gitlabClientBuilder := gitlabClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	resolver := NewGitlabFileUrlResolver(client, gitlabClientBuilder, "https://fake.github.com")
	r1, err := resolver.Resolve(context.TODO(), "https://fake.github.com/foo-user/foo-repo", "myfile", "", &api.SPIAccessToken{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	assert.True(t, gitlabReached)
	assert.Equal(t, "https://fake.github.com/api/v4/projects/foo-user%2Ffoo-repo/repository/files/myfile/raw?ref=HEAD", r1)
}

func TestGetFileHeadGitSuffix(t *testing.T) {
	gitlabReached := false
	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == "https://fake.github.com/api/v4/projects/foo-user%2Ffoo-repo/repository/files/myfile?ref=HEAD" {
				gitlabReached = true
				return &http.Response{
					StatusCode: 200,
					Header: http.Header{
						"content-type":  {"application/json"},
						"x-gitlab-ref":  {"main"},
						"x-gitlab-size": {"6192"},
					},
					Body:    ioutil.NopCloser(strings.NewReader("")),
					Request: r,
				}, nil
			}

			return nil, fmt.Errorf("unexpected request to: %s", r.URL.String())
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
			return &api.Token{
				AccessToken:  "access",
				TokenType:    "fake",
				RefreshToken: "refresh",
				Expiry:       0,
			}, nil
		},
	}
	gitlabClientBuilder := gitlabClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	resolver := NewGitlabFileUrlResolver(client, gitlabClientBuilder, "https://fake.github.com")
	r1, err := resolver.Resolve(context.TODO(), "https://fake.github.com/foo-user/foo-repo.git", "myfile", "", &api.SPIAccessToken{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	assert.True(t, gitlabReached)
	assert.Equal(t, "https://fake.github.com/api/v4/projects/foo-user%2Ffoo-repo/repository/files/myfile/raw?ref=HEAD", r1)
}

func TestGetFileOnBranch(t *testing.T) {
	githubReached := false

	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == "https://fake.github.com/api/v4/projects/foo-user%2Ffoo-repo/repository/files/myfile?ref=v0.1.0" {
				githubReached = true
				return &http.Response{
					StatusCode: 200,
					Header: http.Header{
						"content-type":  {"application/json"},
						"x-gitlab-ref":  {"main"},
						"x-gitlab-size": {"6192"},
					},
					Body:    ioutil.NopCloser(strings.NewReader("")),
					Request: r,
				}, nil
			}

			return nil, fmt.Errorf("unexpected request to: %s", r.URL.String())
		}),
	}
	ts := tokenstorage.TestTokenStorage{
		GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
			return &api.Token{
				AccessToken:  "access",
				TokenType:    "fake",
				RefreshToken: "refresh",
				Expiry:       0,
			}, nil
		},
	}
	gitlabClientBuilder := gitlabClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	resolver := NewGitlabFileUrlResolver(client, gitlabClientBuilder, "https://fake.github.com")
	r1, err := resolver.Resolve(context.TODO(), "https://fake.github.com/foo-user/foo-repo.git", "myfile", "v0.1.0", &api.SPIAccessToken{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	assert.True(t, githubReached)
	assert.Equal(t, "https://fake.github.com/api/v4/projects/foo-user%2Ffoo-repo/repository/files/myfile/raw?ref=v0.1.0", r1)
}

func TestGetUnexistingFile(t *testing.T) {
	mockResponse := "Not Found"

	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 404,
				Header:     http.Header{},
				Body:       ioutil.NopCloser(strings.NewReader(mockResponse)),
				Request:    r,
			}, nil
		}),
	}

	ts := tokenstorage.TestTokenStorage{
		GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
			return &api.Token{
				AccessToken:  "access",
				TokenType:    "fake",
				RefreshToken: "refresh",
				Expiry:       0,
			}, nil
		},
	}
	gitlabClientBuilder := gitlabClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	resolver := NewGitlabFileUrlResolver(client, gitlabClientBuilder, "https://fake.github.com")
	_, err := resolver.Resolve(context.TODO(), "https://fake.github.com/foo-user/foo-repo", "myfile", "efaf08a367921ae130c524db4a531b7696b7d967", &api.SPIAccessToken{})
	if err == nil {
		t.Error("error expected")
	}
	assert.Equal(t, "unexpected status code from GitLab API: 404", fmt.Sprint(err))
}
