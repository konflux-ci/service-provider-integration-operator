package github

import (
	"encoding/json"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var requiredRepositoryRolesForScopes = map[string][]ViewerPermission{
	"admin:repo_hook": {ViewerPermissionAdmin},
}

type tokenFilter struct {
	client client.Client
}

var _ serviceprovider.TokenFilter = (*tokenFilter)(nil)

func (t *tokenFilter) Matches(binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, spState []byte) (bool, error) {
	githubState := TokenState{}
	if err := json.Unmarshal(spState, &githubState); err != nil {
		return false, err
	}

	if token.Status.TokenMetadata == nil {
		return false, nil
	}

	for repoUrl, rec := range githubState {
		if string(repoUrl) == binding.Spec.RepoUrl && permsMatch(&binding.Spec.Permissions, rec, token.Status.TokenMetadata.Scopes) {
			return true, nil
		}
	}

	return false, nil
}

func permsMatch(perms *api.Permissions, rec RepositoryRecord, tokenScopes []string) bool {
	requiredScopes := serviceprovider.GetAllScopes(translateToScopes, perms)

	// how can this NOT be a part of the standard library?
	hasScope := func(scope string) bool {
		for _, s := range tokenScopes {
			if s == scope {
				return true
			}
		}

		return false
	}

	for _, s := range requiredScopes {
		if !hasScope(s) {
			return false
		}
	}

	// ok, so we have all the required scopes... but we also have to check that we have enough permissions in the
	// provided repository
	switch rec.ViewerPermission {
	case ViewerPermissionAdmin:
		return true
	case ViewerPermissionMaintain:

		// TODO implement
		return false
	case ViewerPermissionWrite:
		// TODO implement
		return false
	case ViewerPermissionTriage:
		// TODO implement
		return false
	case ViewerPermissionRead:
		// TODO implement
		return false
	}

	return false
}
