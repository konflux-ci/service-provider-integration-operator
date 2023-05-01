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

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"github.com/google/go-github/v45/github"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type downloadFileCapability struct {
	httpClient      *http.Client
	ghClientBuilder githubClientBuilder
	githubBaseUrl   string
}

var _ serviceprovider.DownloadFileCapability = (*downloadFileCapability)(nil)

var (
	unexpectedStatusCodeError  = errors.New("unexpected status code from GitHub API")
	fileSizeLimitExceededError = errors.New("failed to retrieve file: size too big")
	pathIsADirectoryError      = errors.New("provided path refers to a directory, not file")
	unexpectedRepoUrlError     = errors.New("repoUrl has unexpected format")
)

func (f downloadFileCapability) DownloadFile(ctx context.Context, repoUrl, filepath, ref string, token *api.SPIAccessToken, maxFileSizeLimit int) (string, error) {
	owner, repo, err := f.parseOwnerAndRepoFromUrl(ctx, repoUrl)
	if err != nil {
		return "", fmt.Errorf("could not complile regex to check repoURl: %w", err)
	}
	lg := log.FromContext(ctx)
	ghClient, err := f.ghClientBuilder.createAuthenticatedGhClient(ctx, token)
	if err != nil {
		return "", fmt.Errorf("failed to create authenticated GitHub client: %w", err)
	}
	file, dir, resp, err := ghClient.Repositories.GetContents(ctx, owner, repo, filepath, &github.RepositoryContentGetOptions{Ref: ref})
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

func (f downloadFileCapability) parseOwnerAndRepoFromUrl(ctx context.Context, url string) (owner string, repo string, err error) {
	urlRegexp, err := regexp.Compile(`(?Um)^` + regexp.QuoteMeta(f.githubBaseUrl) + `/(?P<owner>[^/]+)/(?P<repo>[^/]+)(/|(.git)?)$`)
	if err != nil {
		return "", "", fmt.Errorf("could not complile regex to check repoURl: %w", err)
	}

	urlRegexpNames := urlRegexp.SubexpNames()
	matches := urlRegexp.FindAllStringSubmatch(url, -1)
	if len(matches) == 0 {
		return "", "", fmt.Errorf("failed to match github repository: %w", unexpectedRepoUrlError)
	}

	matchesMap := map[string]string{}
	for i, n := range matches[0] {
		matchesMap[urlRegexpNames[i]] = n
	}
	log.FromContext(ctx).V(logs.DebugLevel).Info("parsed values from GitHub URL", "owner", matchesMap["owner"], "repo", matchesMap["repo"])
	return matchesMap["owner"], matchesMap["repo"], nil
}
