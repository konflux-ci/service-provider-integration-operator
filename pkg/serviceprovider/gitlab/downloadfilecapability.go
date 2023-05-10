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
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"net/http"
	"regexp"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/xanzy/go-gitlab"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type downloadFileCapability struct {
	httpClient      *http.Client
	glClientBuilder gitlabClientBuilder
	baseUrl         string
	repoMatcher     gitlabRepoUrlMatcher
}

func NewDownloadFileCapability(httpClient *http.Client, glClientBuilder gitlabClientBuilder, baseUrl string, repoMatcher gitlabRepoUrlMatcher) downloadFileCapability {
	return downloadFileCapability{
		httpClient,
		glClientBuilder,
		baseUrl,
		repoMatcher,
	}
}

var _ serviceprovider.DownloadFileCapability = (*downloadFileCapability)(nil)

var (
	unexpectedStatusCodeError  = errors.New("unexpected status code from GitLab API")
	fileSizeLimitExceededError = errors.New("failed to retrieve file: size too big")
	unexpectedRepoUrlError     = errors.New("repoUrl has unexpected format")
)

func (f downloadFileCapability) DownloadFile(ctx context.Context, repoUrl, filepath, ref string, token *api.SPIAccessToken, maxFileSizeLimit int) (string, error) {
	owner, project, err := f.repoMatcher.parseOwnerAndProjectFromUrl(ctx, repoUrl)
	if err != nil {
		return "", err
	}

	lg := log.FromContext(ctx)
	glClient, err := f.glClientBuilder.createGitlabAuthClient(ctx, token, f.baseUrl)
	if err != nil {
		return "", fmt.Errorf("failed to create authenticated GitLab client: %w", err)
	}

	var refOption gitlab.GetFileOptions

	//ref is required, need to set ir retrieve it
	if ref != "" {
		refOption = gitlab.GetFileOptions{Ref: gitlab.String(ref)}
	} else {
		refOption = gitlab.GetFileOptions{Ref: gitlab.String("HEAD")}
	}

	file, resp, err := glClient.RepositoryFiles.GetFile(owner+"/"+project, filepath, &refOption)
	if err != nil {
		// unfortunately, GitLab library closes the response body, so it is cannot be read
		return "", fmt.Errorf("%w: %d", unexpectedStatusCodeError, resp.StatusCode)
	}

	if file.Size > maxFileSizeLimit {
		lg.Error(err, "file size too big")
		return "", fmt.Errorf("%w: (%d)", fileSizeLimitExceededError, file.Size)
	}
	decoded, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		return "", fmt.Errorf("unable to decode content: %w", err)
	}
	return string(decoded), nil
}

type gitlabRepoUrlMatcher struct {
	regexp  *regexp.Regexp
	baseUrl string
}

func newRepoUrlMatcher(baseUrl string) (gitlabRepoUrlMatcher, error) {
	regex, err := regexp.Compile(`(?Um)^` + baseUrl + `/(?P<owner>[^/]+)/(?P<project>[^/]+)(.git)?$`)
	if err != nil {
		return gitlabRepoUrlMatcher{}, fmt.Errorf("compliling repoUrl matching regex for GitLab baseUrl %s failed with error: %w", baseUrl, err)
	}
	return gitlabRepoUrlMatcher{
		regexp:  regex,
		baseUrl: baseUrl,
	}, nil
}

func (r gitlabRepoUrlMatcher) parseOwnerAndProjectFromUrl(ctx context.Context, repoUrl string) (owner, repo string, err error) {
	urlRegexpNames := r.regexp.SubexpNames()
	matches := r.regexp.FindAllStringSubmatch(repoUrl, -1)
	if len(matches) == 0 {
		return "", "", fmt.Errorf("failed to match GitLab repository with baseUrl %s: %w", r.baseUrl, unexpectedRepoUrlError)
	}

	matchesMap := map[string]string{}
	for i, n := range matches[0] {
		matchesMap[urlRegexpNames[i]] = n
	}
	log.FromContext(ctx).V(logs.DebugLevel).Info("parsed values from GitLab repoUrl",
		"GitLab baseUrl", r.baseUrl, "owner", matchesMap["owner"], "repo", matchesMap["project"])
	return matchesMap["owner"], matchesMap["project"], nil
}
