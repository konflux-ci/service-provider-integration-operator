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
	"errors"
	"fmt"
	"regexp"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/imroc/req"
)

var (
	unexpectedStatusCodeError = errors.New("unexpected status code from GitHub API")
)

type GithubFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int32  `json:"size"`
	Encoding    string `json:"encoding"`
	DownloadUrl string `json:"download_url"`
}

var GithubAPITemplate = "https://api.github.com/repos/%s/%s/contents/%s"
var GithubURLRegexp = regexp.MustCompile(`(?Um)^(?:https)(?:\:\/\/)github.com/(?P<repoUser>[^/]+)/(?P<repoName>[^/]+)(.git)?$`)
var GithubURLRegexpNames = GithubURLRegexp.SubexpNames()

// GitHubScmProvider implements Detector to detect GitHub URLs.
type GitHubScmProvider struct {
}

func (d *GitHubScmProvider) detect(ctx context.Context, repoUrl, filepath, ref string, opts ...interface{}) (bool, string, error) {
	if len(repoUrl) == 0 || !GithubURLRegexp.MatchString(repoUrl) {
		return false, "", nil
	}
	lg := log.FromContext(ctx)

	result := GithubURLRegexp.FindAllStringSubmatch(repoUrl, -1)
	m := map[string]string{}
	for i, n := range result[0] {
		m[GithubURLRegexpNames[i]] = n
	}
	param := req.Param{}
	if ref != "" {
		param["ref"] = ref
		opts = append(opts, param)
	}

	resp, err := req.Get(fmt.Sprintf(GithubAPITemplate, m["repoUser"], m["repoName"], filepath), opts...)
	if err != nil {
		lg.Error(err, "Failed to make GitHub API call")
		return true, "", fmt.Errorf("GitHub API call failed: %w", err)
	}
	res := resp.Response()
	defer res.Body.Close()

	statusCode := res.StatusCode
	lg.V(logs.DebugLevel).Info("GitHub API call response", "status code", statusCode)
	if statusCode >= 400 {
		return true, "", fmt.Errorf("%w: %d. Response: %s", unexpectedStatusCodeError, statusCode, resp.String())
	}

	var file GithubFile
	err = resp.ToJSON(&file)
	if err != nil {
		lg.Error(err, "Failed to parse GitHub json response")
		return true, "", fmt.Errorf("failed to convert GitHub response to JSON: %w", err)
	}
	return true, file.DownloadUrl, nil
}
