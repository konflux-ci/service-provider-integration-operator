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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	unexpectedStatusCodeError  = errors.New("unexpected status code from GitHub API")
	fileSizeLimitExceededError = errors.New("failed to retrieve file: size too big")
)

type GithubFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int32  `json:"size"`
	Encoding    string `json:"encoding"`
	DownloadUrl string `json:"download_url"`
}

var GithubAPITemplate = "https://api.github.com/repos/%s/%s/contents/%s"
var RefTemplate = "?ref=%s"
var GithubURLRegexp = regexp.MustCompile(`(?Um)^(?:https)(?:\:\/\/)github.com/(?P<repoUser>[^/]+)/(?P<repoName>[^/]+)(.git)?$`)
var GithubURLRegexpNames = GithubURLRegexp.SubexpNames()

// GitHubScmProvider implements Detector to detect GitHub URLs.
type GitHubScmProvider struct {
}

func (d *GitHubScmProvider) detect(ctx context.Context, httpClient http.Client, repoUrl, filepath, ref string, authHeaders map[string]string) (bool, string, error) {
	if len(repoUrl) == 0 || !GithubURLRegexp.MatchString(repoUrl) {
		return false, "", nil
	}
	lg := log.FromContext(ctx)

	result := GithubURLRegexp.FindAllStringSubmatch(repoUrl, -1)
	m := map[string]string{}
	for i, n := range result[0] {
		m[GithubURLRegexpNames[i]] = n
	}

	fileUrl := fmt.Sprintf(GithubAPITemplate, m["repoUser"], m["repoName"], filepath)
	if ref != "" {
		fileUrl = fileUrl + fmt.Sprintf(RefTemplate, ref)
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", fileUrl, nil)
	for k, v := range authHeaders {
		req.Header.Add(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		lg.Error(err, "Failed to make GitHub API call")
		return true, "", fmt.Errorf("GitHub API call failed: %w", err)
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	lg.V(logs.DebugLevel).Info("GitHub API call response", "status code", statusCode)

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		lg.Error(err, "failed to read the body of response", "statusCode", resp.StatusCode)
		return true, "", fmt.Errorf("failed to read GitHub file response to JSON: %w", err)
	}
	if statusCode >= 400 {
		return true, "", fmt.Errorf("%w: %d. Response: %s", unexpectedStatusCodeError, statusCode, string(bytes))
	}

	var file GithubFile
	if err := json.Unmarshal(bytes, &file); err != nil {
		lg.Error(err, "failed to unmarshal github file response as JSON")
		return true, "", fmt.Errorf("failed to unmarshal GitHub file response to JSON: %w", err)
	}

	if file.Size > 10485760 {
		lg.Error(err, "file size too big")
		return true, "", fmt.Errorf("%s (%d)", fileSizeLimitExceededError, file.Size)
	}
	return true, file.DownloadUrl, nil
}
