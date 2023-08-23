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
	"errors"
	"fmt"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

var missingTargetError = errors.New("found RemoteSecret does not have a target in the SPIAccessCheck's namespace, this should not happen")

// GenericLookup implements a token lookup in a generic way such that the users only need to provide a function
// to provide a service-provider-specific "state" of the token and a "filter" function that uses the token and its
// state to match it against a binding
type GenericLookup struct {

	// MetadataProvider is used to figure out metadata of a token in the service provider useful for token lookup
	MetadataProvider MetadataProvider
	// MetadataCache is an abstraction used for storing/fetching the metadata of tokens
	MetadataCache *MetadataCache
	//// RepoHostParser is a function that extracts the host from the repoUrl
	//RepoHostParser RepoHostParser

	spiSource          SpiTokenCredentialsSource
	remoteSecretSource RemoteSecretCredentialsSource
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
	token, err := l.spiSource.LookupCredentialSource(ctx, cl, matchable)
	if err != nil {
		return nil, fmt.Errorf("error parsing the host from repo URL %s: %w", matchable.RepoUrl(), err)
	}
	return []api.SPIAccessToken{*token}, nil
}

// TODO remove this method from lookup
func (l GenericLookup) PersistMetadata(ctx context.Context, token *api.SPIAccessToken) error {
	return l.MetadataCache.Ensure(ctx, token, l.MetadataProvider)
}

func (l GenericLookup) LookupCredentials(ctx context.Context, cl client.Client, matchable Matchable) (*Credentials, error) {
	credentials, err := l.remoteSecretSource.LookupCredentials(ctx, cl, matchable)
	if err != nil {
		return nil, fmt.Errorf("error parsing the host from repo URL %s: %w", matchable.RepoUrl(), err)
	}
	if credentials == nil {
		return l.spiSource.LookupCredentials(ctx, cl, matchable)
	}
	return nil, fmt.Errorf("Not found")
}
