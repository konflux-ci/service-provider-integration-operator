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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var emptyBodyError = errors.New("response body is empty")

func readResponseBodyToJsonMap(resp *http.Response) (map[string]interface{}, error) {
	if resp.Body == nil {
		return nil, emptyBodyError
	}
	defer resp.Body.Close()

	var jsonMap map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&jsonMap)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body as JSON: %w", err)
	}

	return jsonMap, nil
}

func doQuayRequest(ctx context.Context, cl rest.HTTPClient, url string, token string, method string, body io.Reader, contentHeader string) (*http.Response, error) {
	lg := log.FromContext(ctx, "url", url)

	lg.Info("asking quay API")

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		lg.Error(err, "failed to compose the request")
		return nil, fmt.Errorf("failed to compose the request to %s: %w", url, err)
	}

	if contentHeader != "" {
		req.Header.Set("Content-Type", contentHeader)
	}
	if token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	}
	resp, err := cl.Do(req)
	if err != nil {
		lg.Error(err, "failed to perform the request")
		return resp, fmt.Errorf("failed to execute a request to %s: %w", url, err)
	}

	return resp, nil
}

func isSuccessfulRequest(ctx context.Context, cl *http.Client, url string, token string) (bool, error) {
	resp, err := doQuayRequest(ctx, cl, url, token, "GET", nil, "")
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

// splitToOrganizationAndRepositoryAndVersion tries to parse the provided repository ID into the organization,
// repository and version parts. It supports both scheme-less docker-like repository ID and URLs of the repositories in
// the quay UI.
func splitToOrganizationAndRepositoryAndVersion(repository string) (string, string, string) {
	schemeIndex := strings.Index(repository, "://")
	if schemeIndex > 0 {
		repository = repository[(schemeIndex + 3):]
	}

	parts := strings.Split(repository, "/")
	if len(parts) < 3 {
		return "", "", ""
	}

	isUIUrl := len(parts) == 4 && parts[1] == "repository"
	if !isUIUrl && len(parts) == 4 {
		return "", "", ""
	}

	host := parts[0]

	if host != "quay.io" {
		return "", "", ""
	}

	repo := parts[1]
	img := parts[2]
	if isUIUrl {
		repo = parts[2]
		img = parts[3]
	}

	imgParts := strings.Split(img, ":")
	version := ""
	if len(imgParts) == 2 {
		version = imgParts[1]
	}

	return repo, imgParts[0], version
}
