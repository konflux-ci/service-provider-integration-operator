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

package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateServiceProviderUrl(t *testing.T) {
	assert.ErrorIs(t, validateServiceProviderUrl("https://foo."), invalidServiceProviderHostError)
	assert.ErrorIs(t, validateServiceProviderUrl("https://Ca$$h.com"), invalidServiceProviderHostError)

	assert.ErrorContains(t, validateServiceProviderUrl("://invalid"), "not parsable")
	assert.ErrorContains(t, validateServiceProviderUrl("https://rick:mory"), "not parsable")

	assert.NoError(t, validateServiceProviderUrl("https://github.com"))
	assert.NoError(t, validateServiceProviderUrl("https://quay.io"))
	assert.NoError(t, validateServiceProviderUrl("http://random.ogre"))
}
