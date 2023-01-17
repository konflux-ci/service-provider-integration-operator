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
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

// GetAllScopes is a helper method to translate all the provided permissions into a list of service-provided-specific
// scopes.
func GetAllScopes(convertToScopes func(permission api.Permission) []string, perms *api.Permissions) []string {
	scopesSet := make(map[string]bool)

	for _, s := range perms.AdditionalScopes {
		scopesSet[s] = true
	}

	for _, p := range perms.Required {
		for _, s := range convertToScopes(p) {
			scopesSet[s] = true
		}
	}

	allScopes := make([]string, 0)
	for s := range scopesSet {
		allScopes = append(allScopes, s)
	}
	return allScopes
}
