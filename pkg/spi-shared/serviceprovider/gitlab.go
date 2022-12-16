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
	"golang.org/x/oauth2"
)

// GitlabEndpoint is the OAuth endpoints specification of SAAS GitLab instance.
var GitlabEndpoint = oauth2.Endpoint{
	AuthURL:  "https://gitlab.com/oauth/authorize",
	TokenURL: "https://gitlab.com/oauth/token",
}

var GitlabSaasHost = "gitlab.com"
