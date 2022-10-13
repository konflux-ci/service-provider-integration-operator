package gitlab

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
)

var _ serviceprovider.TokenFilter = (*tokenFilter)(nil)

type tokenFilter struct {
	metadataProvider *metadataProvider
}

func (t tokenFilter) Matches(ctx context.Context, matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
	lg := log.FromContext(ctx, "matchableUrl", matchable.RepoUrl())

	lg.Info("matching", "token", token.Name)
	if token.Status.TokenMetadata == nil {
		return false, nil
	}

	requiredScopes := serviceprovider.GetAllScopes(translateToGitlabScopes, matchable.Permissions())
	hasScope := func(scope Scope) bool {
		for _, s := range token.Status.TokenMetadata.Scopes {
			if Scope(s).Implies(scope) {
				return true
			}
		}
		return false
	}
	for _, s := range requiredScopes {
		scope := Scope(s)
		if !hasScope(scope) {
			return false, nil
		}
	}

	return true, nil
}
