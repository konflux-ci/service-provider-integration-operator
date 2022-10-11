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
	"context"
	"io"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetFileContents is a main entry function allowing to retrieve file content from the SCM provider.
// It expects three file location parameters, from which the repository URL and path to the file are mandatory,
// and optional Git reference for the branch/tags/commitIds.
// Function type parameter is a callback used when user authentication is needed in order to retrieve the file,
// that function will be called with the URL to OAuth service, where user need to be redirected.
func GetFileContents(ctx context.Context, k8sClient client.Client, httpClient http.Client, namespace, secret, repoUrl, filepath, ref string) (io.ReadCloser, error) {
	authHeaders, err := buildAuthHeader(ctx, k8sClient, repoUrl, namespace, secret)
	if err != nil {
		return nil, err
	}
	fileUrl, err := detect(ctx, httpClient, repoUrl, filepath, ref, authHeaders)
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", fileUrl, nil)
	for k, v := range authHeaders {
		req.Header.Add(k, v)
	}
	response, err := httpClient.Do(req)
	return response.Body, err
}
