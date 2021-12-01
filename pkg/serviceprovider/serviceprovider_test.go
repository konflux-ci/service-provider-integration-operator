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
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestDetectsGithubUrls(t *testing.T) {
	spt, err := TypeFromURL("https://github.com")
	assert.NoError(t, err)
	assert.Equal(t, api.ServiceProviderTypeGitHub, spt)
}

func TestDetectsQuayUrls(t *testing.T) {
	spt, err := TypeFromURL("https://quay.io")
	assert.NoError(t, err)
	assert.Equal(t, api.ServiceProviderTypeQuay, spt)
}

func TestFailsOnUnknownProviderUrl(t *testing.T) {
	_, err := TypeFromURL("https://over.the.rainbow")
	assert.Error(t, err)
}
