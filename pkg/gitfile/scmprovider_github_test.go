// Copyright (c) 2022 Red Hat, Inc.
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

package gitfile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

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

	r1, err := detect(context.TODO(), *client, "https://github.com/foo-user/foo-repo", "myfile", "HEAD", map[string]string{})
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

	r1, err := detect(context.TODO(), *client, "https://github.com/foo-user/foo-repo.git", "myfile", "HEAD", map[string]string{})
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

	r1, err := detect(context.TODO(), *client, "https://github.com/foo-user/foo-repo", "myfile", "v0.1.0", map[string]string{})
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

	r1, err := detect(context.TODO(), *client, "https://github.com/foo-user/foo-repo", "myfile", "efaf08a367921ae130c524db4a531b7696b7d967", map[string]string{})
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

	_, err := detect(context.TODO(), *client, "https://github.com/foo-user/foo-repo", "myfile", "efaf08a367921ae130c524db4a531b7696b7d967", map[string]string{})
	if err == nil {
		t.Error("error expected")
	}
	assert.Equal(t, "detection failed: unexpected status code from GitHub API: 404. Response: {\"message\":\"Not Found\",\"documentation_url\":\"https://docs.github.com/rest/reference/repos#get-repository-content\"}", fmt.Sprint(err))
}
