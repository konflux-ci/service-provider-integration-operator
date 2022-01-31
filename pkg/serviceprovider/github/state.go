package github

import "strings"

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
	ScopeRepo Scope = "repo"
	ScopeRepoStatus Scope = "repo:status"
	ScopeRepoDeployment Scope = "repo_deployment"
	ScopePublicRepo Scope = "public_repo"
	ScopeRepoInvite Scope = "repo:invite"
	ScopeSecurityEvent Scope = "security_event"
	ScopeAdminRepoHook Scope = "admin:repo_hook"
	ScopeWriteRepoHook Scope = "write:repo_hook"
	ScopeReadRepoHook Scope = "read:repo_hook"
	ScopeAdminOrg Scope = "admin:org"
	ScopeWriteOrg Scope = "write:org"
	ScopeReadOrg Scope = "read:org"
	ScopeAdminPublicKey Scope = "admin:public_key"
	ScopeWritePublicKey Scope = "write:public_key"
	ScopeReadPublicKey Scope = "read:public_key"
	ScopeAdminOrgHook Scope = "admin:org_hook"
	ScopeGist Scope = "gist"
	ScopeNotifications Scope = "notifications"
	ScopeUser Scope = "user"
	ScopeReadUser Scope = "read:user"
	ScopeUserEmail Scope = "user:email"
	ScopeUserFollow Scope = "user:follow"
	ScopeDeleteRepo Scope = "delete_repo"
	ScopeWriteDiscussion Scope = "write:discussion"
	ScopeReadDiscussion Scope = "read:discussion"
	ScopeWritePackages Scope = "write:packages"
	ScopeReadPackages Scope = "read:packages"
	ScopeDeletePackages Scope = "delete:packages"
	ScopeAdminGpgKey Scope = "admin:gpg_key"
	ScopeWriteGpgKey Scope = "write:gpg_key"
	ScopeReadGpgKey Scope = "read:gpg_key"
	ScopeCodespace Scope = "codespace"
	ScopeWorkflow Scope = "workflow"
)

type TokenState map[RepositoryUrl]RepositoryRecord

func (s Scope) Implies(other Scope) bool {
	if s == other {
		return true
	}

	// per docs, read:user implies user:email and user:follow
	if s == ScopeReadUser {
		return other == ScopeUserEmail || other == ScopeUserFollow
	}

	// all scopes apart from repo and user-related scopes seem to be organized by admin:, read:, write: prefixes

	sother := string(other)
	otherColonIdx := strings.Index(sother, ":")
	if otherColonIdx < 0 {
		if s == ScopeRepo {
			return other == ScopePublicRepo || strings.HasPrefix(sother, "repo")
		}
		if s == ScopeUser {
			// user implies read:user, but does NOT imply user:email and user:follow
			return other == ScopeReadUser
		}
	}

	otherPrefix := sother[0:otherColonIdx]
	otherSuffix := sother[otherColonIdx+1:]

	ss := string(s)
	sColonIdx := strings.Index(ss, ":")
	if sColonIdx < 0 {
		return false
	}

	sPrefix := ss[0:sColonIdx]
	sSuffix := ss[sColonIdx:]

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

func (vp ViewerPermission) Enables(scope Scope) bool {
	// TODO implement this
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
