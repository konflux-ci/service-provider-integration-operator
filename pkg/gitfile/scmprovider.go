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
	"net/http"
)

var (
	invalidSourceError = errors.New("invalid source string")
)

// ScmProvider defines the interface that in order to determine if URL belongs to SCM provider
type ScmProvider interface {
	// resolve will check whether the provided repository URL matches a known SCM pattern,
	// and resolves input params into valid file download URL.
	// Params are repository, path to the file inside the repository, Git reference (branch/tag/commitId) and
	// set of optional Http headers for authentication
	resolve(ctc context.Context, httpClient http.Client, repoUrl, filepath, ref string, authHeaders map[string]string) (bool, string, error)
}

// ScmProviders is the list of detectors that are tried on an SCM URL.
// This is also the order they're tried (index 0 is first).
var ScmProviders []ScmProvider

func init() {
	ScmProviders = []ScmProvider{
		new(GitHubScmProvider),
	}
}

//detect tries to recognize correct provider for the given repository URL and call it's download URL resolve method
func detect(ctx context.Context, restClient http.Client, repoUrl, filepath, ref string, authHeaders map[string]string) (string, error) {
	for _, d := range ScmProviders {
		ok, resultUrl, err := d.resolve(ctx, restClient, repoUrl, filepath, ref, authHeaders)
		if err != nil {
			return "", fmt.Errorf("detection failed: %w", err)
		}
		if !ok {
			continue
		}
		return resultUrl, nil
	}
	return "", fmt.Errorf("%w: %s for %s", invalidSourceError, repoUrl, filepath)
}
