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
