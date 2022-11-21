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

package serviceprovider

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type FileUrlResolver interface {

	// Resolve will check whether the provided repository URL matches a known SCM pattern,
	// and resolves input params into valid file download URL.
	// Params are repository, path to the file inside the repository, Git reference (branch/tag/commitId)
	// and SPIAccessToken instance to help with authentication
	Resolve(ctc context.Context, repoUrl, filepath, ref string, token *api.SPIAccessToken) (string, error)
}
