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

package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var allScopes = []Scope{
	ScopeRepo,
	ScopeRepoStatus,
	ScopeRepoDeployment,
	ScopePublicRepo,
	ScopeRepoInvite,
	ScopeSecurityEvent,
	ScopeAdminRepoHook,
	ScopeWriteRepoHook,
	ScopeReadRepoHook,
	ScopeAdminOrg,
	ScopeWriteOrg,
	ScopeReadOrg,
	ScopeAdminPublicKey,
	ScopeWritePublicKey,
	ScopeReadPublicKey,
	ScopeAdminOrgHook,
	ScopeGist,
	ScopeNotifications,
	ScopeUser,
	ScopeReadUser,
	ScopeUserEmail,
	ScopeUserFollow,
	ScopeDeleteRepo,
	ScopeWriteDiscussion,
	ScopeReadDiscussion,
	ScopeWritePackages,
	ScopeReadPackages,
	ScopeDeletePackages,
	ScopeAdminGpgKey,
	ScopeWriteGpgKey,
	ScopeReadGpgKey,
	ScopeCodespace,
	ScopeWorkflow,
}

func TestScope_Implies(t *testing.T) {
	testImplies := func(t *testing.T, tested Scope, implied ...Scope) {
		shouldImply := func(s Scope) bool {
			if tested == s {
				return true
			}

			for _, i := range implied {
				if i == s {
					return true
				}
			}

			return false
		}

		for _, s := range allScopes {
			assert.Equal(t, shouldImply(s), tested.Implies(s), "tested %s with %s", tested, s)
		}
	}

	t.Run(string(ScopeRepo), func(t *testing.T) {
		testImplies(t, ScopeRepo, ScopePublicRepo, ScopeRepoInvite, ScopeRepoStatus, ScopeRepoDeployment)
	})

	t.Run(string(ScopeRepoStatus), func(t *testing.T) {
		testImplies(t, ScopeRepoStatus /* nothing implied */)
	})

	t.Run(string(ScopeRepoDeployment), func(t *testing.T) {
		testImplies(t, ScopeRepoDeployment /* nothing implied */)
	})

	t.Run(string(ScopePublicRepo), func(t *testing.T) {
		testImplies(t, ScopePublicRepo /* nothing implied */)
	})

	t.Run(string(ScopeRepoInvite), func(t *testing.T) {
		testImplies(t, ScopeRepoInvite /* nothing implied */)
	})

	t.Run(string(ScopeSecurityEvent), func(t *testing.T) {
		testImplies(t, ScopeSecurityEvent /* nothing implied */)
	})

	t.Run(string(ScopeAdminRepoHook), func(t *testing.T) {
		testImplies(t, ScopeAdminRepoHook, ScopeReadRepoHook, ScopeWriteRepoHook)
	})

	t.Run(string(ScopeWriteRepoHook), func(t *testing.T) {
		testImplies(t, ScopeWriteRepoHook, ScopeReadRepoHook)
	})

	t.Run(string(ScopeReadRepoHook), func(t *testing.T) {
		testImplies(t, ScopeReadRepoHook /* nothing implied */)
	})

	t.Run(string(ScopeAdminOrg), func(t *testing.T) {
		testImplies(t, ScopeAdminOrg, ScopeReadOrg, ScopeWriteOrg)
	})

	t.Run(string(ScopeWriteOrg), func(t *testing.T) {
		testImplies(t, ScopeWriteOrg, ScopeReadOrg)
	})

	t.Run(string(ScopeReadOrg), func(t *testing.T) {
		testImplies(t, ScopeReadOrg /* nothing implied */)
	})

	t.Run(string(ScopeAdminPublicKey), func(t *testing.T) {
		testImplies(t, ScopeAdminPublicKey, ScopeReadPublicKey, ScopeWritePublicKey)
	})

	t.Run(string(ScopeWritePublicKey), func(t *testing.T) {
		testImplies(t, ScopeWritePublicKey, ScopeReadPublicKey)
	})

	t.Run(string(ScopeReadPublicKey), func(t *testing.T) {
		testImplies(t, ScopeReadPublicKey /* nothing implied */)
	})

	t.Run(string(ScopeAdminOrgHook), func(t *testing.T) {
		testImplies(t, ScopeAdminOrgHook /* nothing implied */)
	})

	t.Run(string(ScopeGist), func(t *testing.T) {
		testImplies(t, ScopeGist /* nothing implied */)
	})

	t.Run(string(ScopeNotifications), func(t *testing.T) {
		testImplies(t, ScopeNotifications /* nothing implied */)
	})

	t.Run(string(ScopeUser), func(t *testing.T) {
		testImplies(t, ScopeUser, ScopeReadUser)
	})

	t.Run(string(ScopeReadUser), func(t *testing.T) {
		testImplies(t, ScopeReadUser, ScopeUserEmail, ScopeUserFollow)
	})

	t.Run(string(ScopeUserEmail), func(t *testing.T) {
		testImplies(t, ScopeUserEmail /* nothing implied */)
	})

	t.Run(string(ScopeUserFollow), func(t *testing.T) {
		testImplies(t, ScopeUserFollow /* nothing implied */)
	})

	t.Run(string(ScopeDeleteRepo), func(t *testing.T) {
		testImplies(t, ScopeDeleteRepo /* nothing implied */)
	})

	t.Run(string(ScopeWriteDiscussion), func(t *testing.T) {
		testImplies(t, ScopeWriteDiscussion, ScopeReadDiscussion)
	})

	t.Run(string(ScopeReadDiscussion), func(t *testing.T) {
		testImplies(t, ScopeReadDiscussion /* nothing implied */)
	})

	t.Run(string(ScopeWritePackages), func(t *testing.T) {
		testImplies(t, ScopeWritePackages, ScopeReadPackages)
	})

	t.Run(string(ScopeReadPackages), func(t *testing.T) {
		testImplies(t, ScopeReadPackages /* nothing implied */)
	})

	t.Run(string(ScopeDeletePackages), func(t *testing.T) {
		testImplies(t, ScopeDeletePackages /* nothing implied */)
	})

	t.Run(string(ScopeAdminGpgKey), func(t *testing.T) {
		testImplies(t, ScopeAdminGpgKey, ScopeReadGpgKey, ScopeWriteGpgKey)
	})

	t.Run(string(ScopeWriteGpgKey), func(t *testing.T) {
		testImplies(t, ScopeWriteGpgKey, ScopeReadGpgKey)
	})

	t.Run(string(ScopeReadGpgKey), func(t *testing.T) {
		testImplies(t, ScopeReadGpgKey /* nothing implied */)
	})

	t.Run(string(ScopeCodespace), func(t *testing.T) {
		testImplies(t, ScopeCodespace /* nothing implied */)
	})

	t.Run(string(ScopeWorkflow), func(t *testing.T) {
		testImplies(t, ScopeWorkflow /* nothing implied */)
	})
}

func TestViewerPermission_Enables(t *testing.T) {
	t.Run("admin", func(t *testing.T) {
		for _, s := range allScopes {
			assert.True(t, ViewerPermissionAdmin.Enables(s), "scope", s)
		}
	})

	t.Run("maintain", func(t *testing.T) {
		// TODO implement
		t.Skip()
	})

	t.Run("triage", func(t *testing.T) {
		// TODO implement
		t.Skip()
	})

	t.Run("write", func(t *testing.T) {
		// TODO implement
		t.Skip()
	})

	t.Run("read", func(t *testing.T) {
		// TODO implement
		t.Skip()
	})
}
