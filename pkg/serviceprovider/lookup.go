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
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	// MetadataProvider is used to figure out metadata of a token in the service provider useful for token lookup
	MetadataProvider MetadataProvider
	// MetadataCache is an abstraction used for storing/fetching the metadata of tokens
	MetadataCache *MetadataCache
	// RepoHostParser is a function that extracts the host from the repoUrl
	RepoHostParser RepoHostParser
}

type RepoHostParser func(url string) (string, error)

func RepoHostFromSchemelessUrl(repoUrl string) (string, error) {
	schemeIndex := strings.Index(repoUrl, "://")
	if schemeIndex == -1 {
		repoUrl = "https://" + repoUrl
	}
	return RepoHostFromUrl(repoUrl)
}

func RepoHostFromUrl(repoUrl string) (string, error) {
	parsed, err := url.Parse(repoUrl)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	return parsed.Host, nil
}

func (l GenericLookup) Lookup(ctx context.Context, cl client.Client, matchable Matchable) ([]api.SPIAccessToken, error) {
	lg := log.FromContext(ctx)

	var result = make([]api.SPIAccessToken, 0)

	potentialMatches := &api.SPIAccessTokenList{}

	repoHost, err := l.RepoHostParser(matchable.RepoUrl())
	if err != nil {
		return result, fmt.Errorf("error parsing the host from repo URL %s: %w", matchable.RepoUrl(), err)
	}

	if err := cl.List(ctx, potentialMatches, client.InNamespace(matchable.ObjNamespace()), client.MatchingLabels{
		api.ServiceProviderTypeLabel: string(l.ServiceProviderType),
		api.ServiceProviderHostLabel: repoHost,
	}); err != nil {
		return result, fmt.Errorf("failed to list the potentially matching tokens: %w", err)
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
			if err := l.MetadataCache.Ensure(ctx, &tkn, l.MetadataProvider); err != nil {
				mutex.Lock()
				defer mutex.Unlock()
				lg.Error(err, "failed to refresh the metadata of candidate token", "token", tkn.Namespace+"/"+tkn.Name)
				errs = append(errs, err)
				return
			}

			ok, err := l.TokenFilter.Matches(ctx, matchable, &tkn)
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
		return nil, fmt.Errorf("errors while examining the potential matches: %w", errors.NewAggregate(errs))
	}

	lg.V(logs.DebugLevel).Info("lookup finished", "matching_tokens", len(result))

	return result, nil
}

func (l GenericLookup) PersistMetadata(ctx context.Context, token *api.SPIAccessToken) error {
	return l.MetadataCache.Ensure(ctx, token, l.MetadataProvider)
}
