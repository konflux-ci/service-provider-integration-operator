package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ serviceprovider.ServiceProvider = (*Gitlab)(nil)

type Gitlab struct {
	Configuration    *opconfig.OperatorConfiguration
	lookup           serviceprovider.GenericLookup
	metadataProvider *metadataProvider
	httpClient       rest.HTTPClient
	tokenStorage     tokenstorage.TokenStorage
	baseUrl          string
}

var _ serviceprovider.ConstructorFunc = newGitlab

func newGitlab(factory *serviceprovider.Factory, baseUrl string) (serviceprovider.ServiceProvider, error) {

	// in Quay, we invalidate the individual cached repository records, because we're filling up the cache repo-by-repo
	// therefore the metadata as a whole never gets refreshed.
	cache := serviceprovider.NewMetadataCache(factory.KubernetesClient, &serviceprovider.NeverMetadataExpirationPolicy{})
	mp := &metadataProvider{
		tokenStorage:     factory.TokenStorage,
		httpClient:       factory.HttpClient,
		kubernetesClient: factory.KubernetesClient,
		ttl:              factory.Configuration.TokenLookupCacheTtl,
	}
	return &Gitlab{
		Configuration: factory.Configuration,
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeGitLab,
			TokenFilter:         serviceprovider.NewFilter(factory.Configuration.TokenMatchPolicy, &tokenFilter{}),
			MetadataProvider:    mp,
			MetadataCache:       &cache,
			RepoHostParser:      serviceprovider.RepoHostFromSchemelessUrl,
		},
		tokenStorage:     factory.TokenStorage,
		metadataProvider: mp,
		httpClient:       factory.HttpClient,
		baseUrl:          baseUrl,
	}, nil
}

func (g Gitlab) LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	tokens, err := g.lookup.Lookup(ctx, cl, binding)
	if err != nil {
		return nil, fmt.Errorf("gitlab token lookup failure: %w", err)
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	return &tokens[0], nil
}

func (g Gitlab) PersistMetadata(ctx context.Context, _ client.Client, token *api.SPIAccessToken) error {
	if err := g.lookup.PersistMetadata(ctx, token); err != nil {
		return fmt.Errorf("failed to persiste gitlab metadata: %w", err)
	}
	return nil
}

func (g Gitlab) GetBaseUrl() string {
	return g.BaseUrl
}

func (g Gitlab) OAuthScopesFor(permissions *api.Permissions) []string {
	//TODO implement me
	panic("implement me")
}

func (g Gitlab) GetType() api.ServiceProviderType {
	return api.ServiceProviderTypeGitLab
}

func (g Gitlab) CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error) {
	//TODO implement me
	panic("implement me")
}

func (g Gitlab) GetOAuthEndpoint() string {
	return g.Configuration.BaseUrl + "/gitlab/authenticate"
}

func (g Gitlab) MapToken(ctx context.Context, binding *api.SPIAccessTokenBinding, token *api.SPIAccessToken, tokenData *api.Token) (serviceprovider.AccessTokenMapper, error) {
	//TODO implement me
	panic("implement me")
}

func (g Gitlab) Validate(ctx context.Context, validated serviceprovider.Validated) (serviceprovider.ValidationResult, error) {
	//TODO implement me
	panic("implement me")
}

type gitlabProbe struct{}

var _ serviceprovider.Probe = (*gitlabProbe)(nil)

func (p gitlabProbe) Examine(_ *http.Client, url string) (string, error) {
	if strings.Contains(url, "gitlab") {
		return "gitlab", nil //TODO implement real url
	}
	return "", nil
}
