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

package gitlab

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
)

func TestGetFileHead(t *testing.T) {
	gitlabReached := false
	mockResponse, _ := json.Marshal(map[string]interface{}{
		"name":    "myfile",
		"size":    582,
		"content": base64.StdEncoding.EncodeToString([]byte("abcdefg")),
	})

	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == "https://fake.github.com/api/v4/projects/foo-user%2Ffoo-repo/repository/files/myfile?ref=HEAD" {
				gitlabReached = true
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
	gitlabClientBuilder := gitlabClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	fileCapability := NewDownloadFileCapability(client, gitlabClientBuilder, "https://fake.github.com")
	content, err := fileCapability.DownloadFile(context.TODO(), "https://fake.github.com/foo-user/foo-repo", "myfile", "", &api.SPIAccessToken{}, 1024)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	assert.True(t, gitlabReached)
	assert.Equal(t, "abcdefg", content)
}

func TestGetFileHeadGitSuffix(t *testing.T) {
	gitlabReached := false
	mockResponse, _ := json.Marshal(map[string]interface{}{
		"name":    "myfile",
		"size":    582,
		"content": base64.StdEncoding.EncodeToString([]byte("abcdefg")),
	})
	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == "https://fake.github.com/api/v4/projects/foo-user%2Ffoo-repo/repository/files/myfile?ref=HEAD" {
				gitlabReached = true
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
	gitlabClientBuilder := gitlabClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	fileCapability := NewDownloadFileCapability(client, gitlabClientBuilder, "https://fake.github.com")
	content, err := fileCapability.DownloadFile(context.TODO(), "https://fake.github.com/foo-user/foo-repo.git", "myfile", "", &api.SPIAccessToken{}, 1024)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	assert.True(t, gitlabReached)
	assert.Equal(t, "abcdefg", content)
}

func TestGetFileOnBranch(t *testing.T) {
	githubReached := false
	mockResponse, _ := json.Marshal(map[string]interface{}{
		"name":    "myfile",
		"size":    582,
		"content": base64.StdEncoding.EncodeToString([]byte("abcdefg")),
	})

	client := &http.Client{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == "https://fake.github.com/api/v4/projects/foo-user%2Ffoo-repo/repository/files/myfile?ref=v0.1.0" {
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
	gitlabClientBuilder := gitlabClientBuilder{
		httpClient:   client,
		tokenStorage: ts,
	}

	fileCapability := NewDownloadFileCapability(client, gitlabClientBuilder, "https://fake.github.com")
	content, err := fileCapability.DownloadFile(context.TODO(), "https://fake.github.com/foo-user/foo-repo.git", "myfile", "v0.1.0", &api.SPIAccessToken{}, 1024)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	assert.True(t, githubReached)
	assert.Equal(t, "abcdefg", content)
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

	fileCapability := NewDownloadFileCapability(client, gitlabClientBuilder, "https://fake.github.com")
	_, err := fileCapability.DownloadFile(context.TODO(), "https://fake.github.com/foo-user/foo-repo", "myfile", "efaf08a367921ae130c524db4a531b7696b7d967", &api.SPIAccessToken{}, 1024)
	if err == nil {
		t.Error("error expected")
	}
	assert.Equal(t, "unexpected status code from GitLab API: 404", err.Error())
}
