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

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Credentials struct {
	Username         string
	Password         string
	SourceObjectName string
}

type CredentialsSource[D any] interface {
	// LookupCredentialsSource ...
	LookupCredentialsSource(ctx context.Context, cl client.Client, matchable Matchable) (*D, error)
	// LookupCredentials ...
	LookupCredentials(ctx context.Context, cl client.Client, matchable Matchable) (*Credentials, error)
}

type RepoUrlParser func(url string) (*url.URL, error)

func RepoUrlParserFromSchemalessUrl(repoUrl string) (*url.URL, error) {
	schemeIndex := strings.Index(repoUrl, "://")
	if schemeIndex == -1 {
		repoUrl = "https://" + repoUrl
	}
	return RepoUrlParserFromUrl(repoUrl)
}

func RepoUrlParserFromUrl(repoUrl string) (*url.URL, error) {
	parsed, err := url.Parse(repoUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	return parsed, nil
}
