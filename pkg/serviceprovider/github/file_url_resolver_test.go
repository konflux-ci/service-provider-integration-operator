package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
)

func TestGetFileHead(t *testing.T) {
	githubReached := false
	mockResponse, _ := json.Marshal(map[string]interface{}{
		"name":         "myfile",
		"size":         582,
		"download_url": "https://raw.githubusercontent.com/foo-user/foo-repo/HEAD/myfile",
	})

	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == "https://api.github.com/repos/foo-user/foo-repo/contents/myfile?ref=HEAD" {
				githubReached = true
				return &http.Response{
					StatusCode: 200,
					Header:     http.Header{},
					Body:       ioutil.NopCloser(bytes.NewBuffer(mockResponse)),
					Request:    r,
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
	githubClientBuilder := githubClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	resolver := fileUrlResolver{client, githubClientBuilder}
	r1, err := resolver.Resolve(context.TODO(), "https://github.com/foo-user/foo-repo", "myfile", "HEAD", &api.SPIAccessToken{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	assert.True(t, githubReached)
	assert.Equal(t, "https://raw.githubusercontent.com/foo-user/foo-repo/HEAD/myfile", r1)
}

func TestGetFileHeadGitSuffix(t *testing.T) {
	githubReached := false
	mockResponse, _ := json.Marshal(map[string]interface{}{
		"name":         "myfile",
		"size":         582,
		"download_url": "https://raw.githubusercontent.com/foo-user/foo-repo/HEAD/myfile",
	})

	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == "https://api.github.com/repos/foo-user/foo-repo/contents/myfile?ref=HEAD" {
				githubReached = true
				return &http.Response{
					StatusCode: 200,
					Header:     http.Header{},
					Body:       ioutil.NopCloser(bytes.NewBuffer(mockResponse)),
					Request:    r,
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
	githubClientBuilder := githubClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	resolver := fileUrlResolver{client, githubClientBuilder}
	r1, err := resolver.Resolve(context.TODO(), "https://github.com/foo-user/foo-repo.git", "myfile", "HEAD", &api.SPIAccessToken{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	assert.True(t, githubReached)
	assert.Equal(t, "https://raw.githubusercontent.com/foo-user/foo-repo/HEAD/myfile", r1)
}

func TestGetFileOnBranch(t *testing.T) {
	githubReached := false
	mockResponse, _ := json.Marshal(map[string]interface{}{
		"name":         "myfile",
		"size":         582,
		"download_url": "https://raw.githubusercontent.com/foo-user/foo-repo/v0.1.0/myfile",
	})

	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == "https://api.github.com/repos/foo-user/foo-repo/contents/myfile?ref=v0.1.0" {
				githubReached = true
				return &http.Response{
					StatusCode: 200,
					Header:     http.Header{},
					Body:       ioutil.NopCloser(bytes.NewBuffer(mockResponse)),
					Request:    r,
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
	githubClientBuilder := githubClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	resolver := fileUrlResolver{client, githubClientBuilder}
	r1, err := resolver.Resolve(context.TODO(), "https://github.com/foo-user/foo-repo", "myfile", "v0.1.0", &api.SPIAccessToken{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	assert.True(t, githubReached)
	assert.Equal(t, "https://raw.githubusercontent.com/foo-user/foo-repo/v0.1.0/myfile", r1)
}

func TestGetFileOnCommitId(t *testing.T) {

	githubReached := false
	mockResponse, _ := json.Marshal(map[string]interface{}{
		"name":         "myfile",
		"size":         582,
		"download_url": "https://raw.githubusercontent.com/foo-user/foo-repo/efaf08a367921ae130c524db4a531b7696b7d967/myfile",
	})

	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == "https://api.github.com/repos/foo-user/foo-repo/contents/myfile?ref=efaf08a367921ae130c524db4a531b7696b7d967" {
				githubReached = true
				return &http.Response{
					StatusCode: 200,
					Header:     http.Header{},
					Body:       ioutil.NopCloser(bytes.NewBuffer(mockResponse)),
					Request:    r,
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
	githubClientBuilder := githubClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	resolver := fileUrlResolver{client, githubClientBuilder}
	r1, err := resolver.Resolve(context.TODO(), "https://github.com/foo-user/foo-repo", "myfile", "efaf08a367921ae130c524db4a531b7696b7d967", &api.SPIAccessToken{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	assert.True(t, githubReached)
	assert.Equal(t, "https://raw.githubusercontent.com/foo-user/foo-repo/efaf08a367921ae130c524db4a531b7696b7d967/myfile", r1)
}

func TestGetUnexistingFile(t *testing.T) {
	mockResponse := "{\"message\":\"Not Found\",\"documentation_url\":\"https://docs.github.com/rest/reference/repos#get-repository-content\"}"

	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 404,
				Header:     http.Header{},
				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(mockResponse))),
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
	githubClientBuilder := githubClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	resolver := fileUrlResolver{client, githubClientBuilder}
	_, err := resolver.Resolve(context.TODO(), "https://github.com/foo-user/foo-repo", "myfile", "efaf08a367921ae130c524db4a531b7696b7d967", &api.SPIAccessToken{})
	if err == nil {
		t.Error("error expected")
	}
	assert.Equal(t, "unexpected status code from GitHub API: 404. Response: {\"message\":\"Not Found\",\"documentation_url\":\"https://docs.github.com/rest/reference/repos#get-repository-content\"}", fmt.Sprint(err))
}
