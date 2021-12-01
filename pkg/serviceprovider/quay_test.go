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

	"github.com/stretchr/testify/assert"
)

func TestQuayServiceProviderUrlFromRepo(t *testing.T) {
	q := Quay{Client: nil}
	spUrl, err := q.GetServiceProviderUrlForRepo("https://quay.io/kachny/husy")
	assert.NoError(t, err)
	assert.Equal(t, "https://quay.io", spUrl)
}

func TestQuayServiceProviderUrlFromWrongRepo(t *testing.T) {
	g := Github{Client: nil}
	_, err := g.GetServiceProviderUrlForRepo("https://quay.ganymedes/kachny/husy")
	assert.Error(t, err)
}

// The lookup is tested in the integration tests because it needs interaction with k8s.
