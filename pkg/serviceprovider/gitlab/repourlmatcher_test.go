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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseOwnerAndProjectFromUrl(t *testing.T) {
	matcher, err := newRepoUrlMatcher("https://personal.gitlab.com")
	assert.NoError(t, err)

	testSuccess := func(t *testing.T, repoUrl, expectedOwner, expectedRepo string) {
		owner, repo, err := matcher.parseOwnerAndProjectFromUrl(context.TODO(), repoUrl)

		assert.Equal(t, expectedOwner, owner)
		assert.Equal(t, expectedRepo, repo)
		assert.NoError(t, err)
	}
	testFail := func(t *testing.T, repoUrl string) {
		_, _, err := matcher.parseOwnerAndProjectFromUrl(context.TODO(), repoUrl)
		assert.Error(t, err)
	}

	testSuccess(t, "https://personal.gitlab.com/foo-user/foo-project", "foo-user", "foo-project")
	testSuccess(t, "https://personal.gitlab.com/foo-user/foo-project/", "foo-user", "foo-project")
	testSuccess(t, "https://personal.gitlab.com/foo-user/foo-project.git", "foo-user", "foo-project")

	testFail(t, "https://personal.gitlab.com/foo-user/foo-repo/.git/")
	testFail(t, "https://personal.gitlab.com/foo-user/foo-repo/.git")
	testFail(t, "https://personal.gitlab.com/with/more/path/splits")
	testFail(t, "https://personal.gitlab.com/owner/")
	testFail(t, "https://personal.gitlab.com")
	testFail(t, "https://gitlab.com/owner/repo")
	testFail(t, "https://github.com/owner/repo")
	testFail(t, "duck")

}
