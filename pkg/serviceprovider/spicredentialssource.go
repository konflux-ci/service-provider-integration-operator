package serviceprovider

import (
	"context"

	kubeerrors "k8s.io/apimachinery/pkg/util/errors"

	"errors"
	"fmt"
	"sync"

	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var accessTokenNotFoundError = errors.New("token data is not found in token storage")

var _ CredentialsSource[api.SPIAccessToken] = (*SPIAccessTokenCredentialsSource)(nil)

type SPIAccessTokenCredentialsSource struct {
	// ServiceProviderType is just the type of the provider we're dealing with. It is used to limit the number of
	// results the filter function needs to sift through.
	ServiceProviderType api.ServiceProviderType
	// TokenFilter is the filter function that decides whether a token matches the requirements of a binding, given
	// the token's service-provider-specific state
	TokenFilter TokenFilter
	// MetadataProvider is used to figure out metadata of a token in the service provider useful for token lookup
	MetadataProvider MetadataProvider
	// MetadataCache is an abstraction used for storing/fetching the metadata of tokens
	MetadataCache *MetadataCache
	// RepoHostParser is a function that extracts the host from the repoUrl
	RepoHostParser RepoUrlParser
	TokenStorage   tokenstorage.TokenStorage
}

func (s SPIAccessTokenCredentialsSource) LookupCredentialsSource(ctx context.Context, cl client.Client, matchable Matchable) (*api.SPIAccessToken, error) {
	lg := log.FromContext(ctx)

	var result = make([]api.SPIAccessToken, 0)

	potentialMatches := &api.SPIAccessTokenList{}

	repoUrl, err := s.RepoHostParser(matchable.RepoUrl())
	if err != nil {
		return nil, fmt.Errorf("error parsing the host from repo URL %s: %w", matchable.RepoUrl(), err)
	}

	if err := cl.List(ctx, potentialMatches, client.InNamespace(matchable.ObjNamespace()), client.MatchingLabels{
		api.ServiceProviderTypeLabel: string(s.ServiceProviderType),
		api.ServiceProviderHostLabel: repoUrl.Host,
	}); err != nil {
		return nil, fmt.Errorf("failed to list the potentially matching tokens: %w", err)
	}

	lg.V(logs.DebugLevel).Info("lookup", "potential_matches", len(potentialMatches.Items))

	errs := make([]error, 0)

	mutex := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, t := range potentialMatches.Items {
		if t.Status.Phase != api.SPIAccessTokenPhaseReady {
			lg.V(logs.DebugLevel).Info("skipping lookup, token not ready", "token", t.Name)
			continue
		}

		wg.Add(1)
		go func(tkn api.SPIAccessToken) {
			lg.V(logs.DebugLevel).Info("matching", "token", tkn.Name)
			defer wg.Done()
			if err := s.MetadataCache.Ensure(ctx, &tkn, s.MetadataProvider); err != nil {
				mutex.Lock()
				defer mutex.Unlock()
				lg.Error(err, "failed to refresh the metadata of candidate token", "token", tkn.Namespace+"/"+tkn.Name)
				errs = append(errs, err)
				return
			}

			ok, err := s.TokenFilter.Matches(ctx, matchable, &tkn)
			if err != nil {
				mutex.Lock()
				defer mutex.Unlock()
				lg.Error(err, "failed to match candidate token", "token", tkn.Namespace+"/"+tkn.Name)
				errs = append(errs, err)
				return
			}
			if ok {
				mutex.Lock()
				defer mutex.Unlock()
				result = append(result, tkn)
			}
		}(t)
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("errors while examining the potential matches: %w", kubeerrors.NewAggregate(errs))
	}
	lg.V(logs.DebugLevel).Info("lookup finished", "matching_tokens", len(result))
	if len(result) > 0 {
		return &result[0], nil
	}
	return nil, nil
}
func (s SPIAccessTokenCredentialsSource) LookupCredentials(ctx context.Context, cl client.Client, matchable Matchable) (*Credentials, error) {
	lg := log.FromContext(ctx)
	spiToken, err := s.LookupCredentialsSource(ctx, cl, matchable)
	if err != nil {
		return nil, fmt.Errorf("unable to find suitable matching SPIAccessToken: %w", err)
	}
	if spiToken == nil {
		return nil, nil
	}

	tokenData, tsErr := s.TokenStorage.Get(ctx, spiToken)
	if tsErr != nil {
		lg.Error(tsErr, "failed to get token from storage for", "token", spiToken)
		return nil, fmt.Errorf("failed to get token from storage for %s/%s: %w", spiToken.Namespace, spiToken.Name, tsErr)
	}

	if tokenData == nil {
		lg.Error(accessTokenNotFoundError, "token data not found", "token-name", spiToken.Name)
		return nil, accessTokenNotFoundError
	}

	return &Credentials{Username: tokenData.Username, Password: tokenData.AccessToken}, nil
}
