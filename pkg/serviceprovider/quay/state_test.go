package quay

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var allScopes = []Scope{
	ScopeRepoRead,
	ScopeRepoWrite,
	ScopeRepoAdmin,
	ScopeRepoCreate,
	ScopeUserRead,
	ScopeUserAdmin,
	ScopeOrgAdmin,
}

func TestScope_Implies(t *testing.T) {
	testImplies := func(t *testing.T, scope Scope, impliedScopes ...Scope) {
		shouldImply := func(s Scope) bool {
			if scope == s {
				return true
			}

			for _, i := range impliedScopes {
				if i == s {
					return true
				}
			}

			return false
		}

		for _, s := range allScopes {
			assert.Equal(t, shouldImply(s), scope.Implies(s), "tested %s with %s", scope, s)
		}
	}

	t.Run(string(ScopeRepoRead), func(t *testing.T) {
		testImplies(t, ScopeRepoRead /* nothing implied */)
	})

	t.Run(string(ScopeRepoWrite), func(t *testing.T) {
		testImplies(t, ScopeRepoWrite, ScopeRepoRead)
	})

	t.Run(string(ScopeRepoAdmin), func(t *testing.T) {
		testImplies(t, ScopeRepoAdmin, ScopeRepoWrite, ScopeRepoRead, ScopeRepoCreate)
	})

	t.Run(string(ScopeRepoCreate), func(t *testing.T) {
		testImplies(t, ScopeRepoCreate /* nothing implied */)
	})

	t.Run(string(ScopeUserRead), func(t *testing.T) {
		testImplies(t, ScopeUserRead /* nothing implied */)
	})

	t.Run(string(ScopeUserAdmin), func(t *testing.T) {
		testImplies(t, ScopeUserAdmin, ScopeUserRead)
	})

	t.Run(string(ScopeOrgAdmin), func(t *testing.T) {
		testImplies(t, ScopeOrgAdmin /* nothing implied */)
	})
}

func TestScope_IsIncluded(t *testing.T) {
	t.Run("handles implied scopes", func(t *testing.T) {
		assert.True(t, ScopeRepoRead.IsIncluded([]Scope{ScopeRepoWrite, ScopeUserAdmin}))
	})

	t.Run("simple equality", func(t *testing.T) {
		assert.True(t, ScopeRepoWrite.IsIncluded([]Scope{ScopeRepoWrite, ScopeUserAdmin}))
	})
}

func TestFetchRepositoryRecord(t *testing.T) {
	// TODO implement
}

func TestFetchOrganizationRecord(t *testing.T) {
	// TODO implement
}

func TestFetchUserRecord(t *testing.T) {
	// TODO implement
}
