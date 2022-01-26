package serviceprovider

import (
	"context"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

// GenericLookup implements a token lookup in a generic way such that the users only need to provide a function
// to provide a service-provider-specific "state" of the token and a "filter" function that uses the token and its
// state to match it against a binding
type GenericLookup struct {
	// ServiceProviderType is just the type of the provider we're dealing with. It is used to limit the number of
	// results the filter function needs to sift through.
	ServiceProviderType api.ServiceProviderType
	// TokenFilter is the filter function that decides whether a token matches the requirements of a binding, given
	// the token's service-provider-specific state
	TokenFilter TokenFilter
	// StateSerializer is used to figure out a "state" of a token in the service provider useful for token lookup
	StateSerializer StateSerializer
	// StateCache is an abstraction used for storing/fetching the state of tokens
	StateCache *StateCache
}

func (l GenericLookup) Lookup(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) ([]api.SPIAccessToken, error) {
	var result []api.SPIAccessToken

	potentialMatches := &api.SPIAccessTokenList{}

	repoUrl, err := url.Parse(binding.Spec.RepoUrl)
	if err != nil {
		return result, err
	}

	if err := cl.List(ctx, potentialMatches, client.InNamespace(binding.Namespace), client.MatchingLabels{
		api.ServiceProviderTypeLabel: string(l.ServiceProviderType),
		api.ServiceProviderHostLabel: repoUrl.Host,
	}); err != nil {
		return result, err
	}

	res := make(chan *api.SPIAccessToken)
	wg := sync.WaitGroup{}
	for _, t := range potentialMatches.Items {
		wg.Add(1)
		t := t // this is what intellij says is necessary to avoid "unexpected values"
		go func() {
			defer wg.Done()
			state, err := l.StateCache.Ensure(ctx, &t, l.StateSerializer)
			if err != nil {
				return
			}
			ok, err := l.TokenFilter.Matches(binding, &t, state)
			if err != nil {
				return
			}
			if ok {
				res <- &t
			}
		}()
	}

	wg.Wait()
	for included := range res {
		result = append(result, *included)
	}

	return result, nil
}
