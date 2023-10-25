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
	"fmt"
	"regexp"

	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type gitlabRepoUrlMatcher struct {
	regexp  *regexp.Regexp
	baseUrl string
}

func newRepoUrlMatcher(baseUrl string) (gitlabRepoUrlMatcher, error) {
	regex, err := regexp.Compile(`(?Um)^` + regexp.QuoteMeta(baseUrl) + `/(?P<owner>[^/]+)/(?P<project>[^/]+)(/|(.git)?)$`)
	if err != nil {
		return gitlabRepoUrlMatcher{}, fmt.Errorf("compliling repoUrl matching regexp for GitLab baseUrl %s failed with error: %w", baseUrl, err)
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
