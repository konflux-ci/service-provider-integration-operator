package snyk

import (
	"context"
	"net/http"
	"strings"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ serviceprovider.ServiceProvider = (*Snyk)(nil)

type Snyk struct {
	Configuration config.Configuration
	lookup        serviceprovider.GenericLookup
	httpClient    rest.HTTPClient
}

var Initializer = serviceprovider.Initializer{
	Probe:       snykProbe{},
	Constructor: serviceprovider.ConstructorFunc(newSnyk),
}

func newSnyk(factory *serviceprovider.Factory, _ string) (serviceprovider.ServiceProvider, error) {

	cache := serviceprovider.NewMetadataCache(factory.KubernetesClient, &serviceprovider.NeverMetadataExpirationPolicy{})

	return &Snyk{
		Configuration: factory.Configuration,
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeSnyk,
			TokenFilter:         &tokenFilter{},
			MetadataProvider: &metadataProvider{
				httpClient:   factory.HttpClient,
				tokenStorage: factory.TokenStorage,
			},
			MetadataCache: &cache,
			RepoHostParser: serviceprovider.RepoHostParserFunc(func(repoUrl string) (string, error) {
				return repoUrl, nil
			}),
		},
		httpClient: factory.HttpClient,
	}, nil
}

var _ serviceprovider.ConstructorFunc = newSnyk

func (g *Snyk) GetOAuthEndpoint() string {
	return ""
}

func (g *Snyk) GetBaseUrl() string {
	return "https://snyk.io"
}

func (g *Snyk) GetType() api.ServiceProviderType {
	return api.ServiceProviderTypeSnyk
}

func (g *Snyk) TranslateToScopes(_ api.Permission) []string {
	return []string{}
}

func (g *Snyk) LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	tokens, err := g.lookup.Lookup(ctx, cl, binding)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	return &tokens[0], nil
}

func (g *Snyk) PersistMetadata(ctx context.Context, _ client.Client, token *api.SPIAccessToken) error {
	return g.lookup.PersistMetadata(ctx, token)
}

func (g *Snyk) GetServiceProviderUrlForRepo(repoUrl string) (string, error) {
	return serviceprovider.GetHostWithScheme(repoUrl)
}

func (g *Snyk) CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error) {
	log.FromContext(ctx).Info("trying SPIAccessCheck on snyk.io. This is not supported yet.")
	return &api.SPIAccessCheckStatus{
		Accessibility: api.SPIAccessCheckAccessibilityUnknown,
		ErrorReason:   api.SPIAccessCheckErrorNotImplemented,
		ErrorMessage:  "Access check for snyk.io is not implemented.",
	}, nil
}

func (g *Snyk) MapToken(_ context.Context, _ *api.SPIAccessTokenBinding, token *api.SPIAccessToken, tokenData *api.Token) (serviceprovider.AccessTokenMapper, error) {
	return serviceprovider.DefaultMapToken(token, tokenData)
}

func (g *Snyk) Validate(ctx context.Context, _ serviceprovider.Validated) (serviceprovider.ValidationResult, error) {
	return serviceprovider.ValidationResult{}, nil
}

type snykProbe struct{}

var _ serviceprovider.Probe = (*snykProbe)(nil)

func (q snykProbe) Examine(_ *http.Client, url string) (string, error) {
	if strings.HasPrefix(url, "https://snyk.io") || strings.HasPrefix(url, "https://api.snyk.io") {
		return "https://snyk.io", nil
	} else {
		return "", nil
	}
}
