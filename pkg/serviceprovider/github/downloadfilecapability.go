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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/google/go-github/v45/github"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type downloadFileCapability struct {
	httpClient      *http.Client
	ghClientBuilder githubClientBuilder
}

var _ serviceprovider.DownloadFileCapability = (*downloadFileCapability)(nil)

var (
	unexpectedStatusCodeError  = errors.New("unexpected status code from GitHub API")
	fileSizeLimitExceededError = errors.New("failed to retrieve file: size too big")
	pathIsADirectoryError      = errors.New("provided path refers to a directory, not file")
)

var _URLRegexp = regexp.MustCompile(`(?Um)^(?:https)(?:\:\/\/)github.com/(?P<owner>[^/]+)/(?P<repo>[^/]+)(.git)?$`)
var _URLRegexpNames = _URLRegexp.SubexpNames()

func (f downloadFileCapability) DownloadFile(ctx context.Context, repoUrl, filepath, ref string, token *api.SPIAccessToken, maxFileSizeLimit int) (string, error) {
	submatches := _URLRegexp.FindAllStringSubmatch(repoUrl, -1)
	matchesMap := map[string]string{}
	for i, n := range submatches[0] {
		matchesMap[_URLRegexpNames[i]] = n
	}
	lg := log.FromContext(ctx)
	ghClient, err := f.ghClientBuilder.createAuthenticatedGhClient(ctx, token)
	if err != nil {
		return "", fmt.Errorf("failed to create authenticated GitHub client: %w", err)
	}
	file, dir, resp, err := ghClient.Repositories.GetContents(ctx, matchesMap["owner"], matchesMap["repo"], filepath, &github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		checkRateLimitError(err)
		bytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%w: %d. Response: %s", unexpectedStatusCodeError, resp.StatusCode, string(bytes))
	}
	if file == nil && dir != nil {
		return "", fmt.Errorf("%w: %s", pathIsADirectoryError, filepath)
	}
	if file.GetSize() > maxFileSizeLimit {
		lg.Error(err, "file size too big")
		return "", fmt.Errorf("%w: (%d)", fileSizeLimitExceededError, file.Size)
	}
	content, err := file.GetContent()
	if err != nil {
		lg.Error(err, "file content reading error")
		return "", fmt.Errorf("content reading error: %w", err)
	}
	return content, nil
}
