package github

import (
	"encoding/json"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

	hasScope := func(scope Scope) bool {
		for _, s := range tokenScopes {
			if Scope(s).Implies(scope) {
				return true
			}
		}

		return false
	}

	for _, s := range requiredScopes {
		scope := Scope(s)
		if !hasScope(scope) {
			return false
		}

		if !rec.ViewerPermission.Enables(scope) {
			return false
		}
	}

	return true
}
