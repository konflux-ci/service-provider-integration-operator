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
	"strings"
)

type RepositoryUrl string

type RepositoryRecord struct {
	ViewerPermission ViewerPermission `json:"viewerPermission"`
}

type ViewerPermission string
type Scope string

const (
	ViewerPermissionAdmin    ViewerPermission = "ADMIN"
	ViewerPermissionMaintain ViewerPermission = "MAINTAIN"
	ViewerPermissionWrite    ViewerPermission = "WRITE"
	ViewerPermissionTriage   ViewerPermission = "TRIAGE"
	ViewerPermissionRead     ViewerPermission = "READ"
	ScopeRepo                Scope            = "repo"
	ScopeRepoStatus          Scope            = "repo:status"
	ScopeRepoDeployment      Scope            = "repo_deployment"
	ScopePublicRepo          Scope            = "public_repo"
	ScopeRepoInvite          Scope            = "repo:invite"
	ScopeSecurityEvent       Scope            = "security_event"
	ScopeAdminRepoHook       Scope            = "admin:repo_hook"
	ScopeWriteRepoHook       Scope            = "write:repo_hook"
	ScopeReadRepoHook        Scope            = "read:repo_hook"
	ScopeAdminOrg            Scope            = "admin:org"
	ScopeWriteOrg            Scope            = "write:org"
	ScopeReadOrg             Scope            = "read:org"
	ScopeAdminPublicKey      Scope            = "admin:public_key"
	ScopeWritePublicKey      Scope            = "write:public_key"
	ScopeReadPublicKey       Scope            = "read:public_key"
	ScopeAdminOrgHook        Scope            = "admin:org_hook"
	ScopeGist                Scope            = "gist"
	ScopeNotifications       Scope            = "notifications"
	ScopeUser                Scope            = "user"
	ScopeReadUser            Scope            = "read:user"
	ScopeUserEmail           Scope            = "user:email"
	ScopeUserFollow          Scope            = "user:follow"
	ScopeDeleteRepo          Scope            = "delete_repo"
	ScopeWriteDiscussion     Scope            = "write:discussion"
	ScopeReadDiscussion      Scope            = "read:discussion"
	ScopeWritePackages       Scope            = "write:packages"
	ScopeReadPackages        Scope            = "read:packages"
	ScopeDeletePackages      Scope            = "delete:packages"
	ScopeAdminGpgKey         Scope            = "admin:gpg_key"
	ScopeWriteGpgKey         Scope            = "write:gpg_key"
	ScopeReadGpgKey          Scope            = "read:gpg_key"
	ScopeCodespace           Scope            = "codespace"
	ScopeWorkflow            Scope            = "workflow"
)

type TokenState struct {
	AccessibleRepos map[RepositoryUrl]RepositoryRecord
}

func (s Scope) Implies(other Scope) bool {
	if s == other {
		return true
	}

	sother := string(other)

	// per docs, read:user implies user:email and user:follow
	if s == ScopeReadUser {
		return other == ScopeUserEmail || other == ScopeUserFollow
	}
	if s == ScopeRepo {
		return other == ScopePublicRepo || strings.HasPrefix(sother, "repo")
	}
	if s == ScopeUser {
		// user implies read:user, but does NOT imply user:email and user:follow
		return other == ScopeReadUser
	}

	// all scopes apart from the above seem to be organized by admin:, read:, write: prefixes

	ss := string(s)
	sColonIdx := strings.Index(ss, ":")
	if sColonIdx < 0 {
		return false
	}

	sPrefix := ss[0:sColonIdx]
	sSuffix := ss[sColonIdx+1:]

	otherColonIdx := strings.Index(sother, ":")
	if otherColonIdx < 0 {
		return false
	}

	otherPrefix := sother[0:otherColonIdx]
	otherSuffix := sother[otherColonIdx+1:]

	if sSuffix != otherSuffix {
		return false
	}

	if sPrefix == "admin" {
		return true
	} else if sPrefix == "write" {
		return otherPrefix == "read"
	}
	return false
}

func IsValidScope(scope string) bool {
	switch Scope(scope) {
	case ScopeRepo,
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
		ScopeWorkflow:
		return true
	default:
		return false
	}
}

func (vp ViewerPermission) Enables(scope Scope) bool {
	// TODO implement this with https://issues.redhat.com/projects/SVPI/issues/SVPI-71
	switch vp {
	case ViewerPermissionAdmin:
		return true
	case ViewerPermissionMaintain:
		return true
	case ViewerPermissionTriage:
		return true
	case ViewerPermissionWrite:
		return true
	case ViewerPermissionRead:
		return true
	}

	return false
}
