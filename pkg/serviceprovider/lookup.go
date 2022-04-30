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
	"net/url"
	"sync"

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
}

func (l GenericLookup) Lookup(ctx context.Context, cl client.Client, matchable Matchable) ([]api.SPIAccessToken, error) {
	var result = make([]api.SPIAccessToken, 0)

	potentialMatches := &api.SPIAccessTokenList{}

	repoUrl, err := url.Parse(matchable.RepoUrl())
	if err != nil {
		return result, err
	}

	if err := cl.List(ctx, potentialMatches, client.InNamespace(matchable.ObjNamespace()), client.MatchingLabels{
		api.ServiceProviderTypeLabel: string(l.ServiceProviderType),
		api.ServiceProviderHostLabel: repoUrl.Host,
	}); err != nil {
		return result, err
	}

	lg := log.FromContext(ctx)

	errs := make([]error, 0)

	mutex := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, t := range potentialMatches.Items {
		if t.Status.Phase != api.SPIAccessTokenPhaseReady {
			continue
		}
		wg.Add(1)
		go func(tkn api.SPIAccessToken) {
			defer wg.Done()
			if err := l.MetadataCache.Ensure(ctx, &tkn, l.MetadataProvider); err != nil {
				mutex.Lock()
				defer mutex.Unlock()
				lg.Error(err, "failed to refresh the metadata of candidate token", "token", tkn.Namespace+"/"+tkn.Name)
				errs = append(errs, err)
				return
			}

			ok, err := l.TokenFilter.Matches(matchable, &tkn)
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
		return nil, errors.NewAggregate(errs)
	}

	return result, nil
}

func (l GenericLookup) PersistMetadata(ctx context.Context, token *api.SPIAccessToken) error {
	return l.MetadataCache.Ensure(ctx, token, l.MetadataProvider)
}
