package common

import (
	"context"
	"strings"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Common struct {
	Configuration config.Configuration
	lookup        serviceprovider.GenericLookup
	httpClient    rest.HTTPClient
	repoUrl       string
}

var Initializer = serviceprovider.Initializer{
	Constructor: serviceprovider.ConstructorFunc(newCommon),
}

func newCommon(factory *serviceprovider.Factory, repoUrl string) (serviceprovider.ServiceProvider, error) {

	cache := serviceprovider.NewMetadataCache(factory.KubernetesClient, &serviceprovider.NeverMetadataExpirationPolicy{})
	return &Common{
		Configuration: factory.Configuration,
		lookup: serviceprovider.GenericLookup{
			ServiceProviderType: api.ServiceProviderTypeCommon,
			TokenFilter:         &tokenFilter{},
			RepoHostParser: serviceprovider.RepoHostParserFunc(func(repoUrl string) (string, error) {
				schemeIndex := strings.Index(repoUrl, "://")
				if schemeIndex == -1 {
					repoUrl = "https://" + repoUrl
				}

				return serviceprovider.RepoHostFromUrl(repoUrl)
			}),
			MetadataCache: &cache,
			MetadataProvider: &metadataProvider{
				tokenStorage: factory.TokenStorage,
			},
		},
		httpClient: factory.HttpClient,
		repoUrl:    repoUrl,
	}, nil
}

var _ serviceprovider.ConstructorFunc = newCommon

func (g *Common) GetOAuthEndpoint() string {
	return ""
}

func (g *Common) GetBaseUrl() string {
	base, _ := serviceprovider.GetHostWithScheme(g.repoUrl)
	return base
}

func (g *Common) GetType() api.ServiceProviderType {
	return api.ServiceProviderTypeCommon
}

func (g *Common) TranslateToScopes(_ api.Permission) []string {
	return []string{}
}

func (g *Common) LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	tokens, err := g.lookup.Lookup(ctx, cl, binding)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	return &tokens[0], nil
}

func (g *Common) PersistMetadata(ctx context.Context, _ client.Client, token *api.SPIAccessToken) error {
	return g.lookup.PersistMetadata(ctx, token)
}

func (g *Common) GetServiceProviderUrlForRepo(repoUrl string) (string, error) {
	return serviceprovider.GetHostWithScheme(repoUrl)
}

func (g *Common) CheckRepositoryAccess(ctx context.Context, cl client.Client, accessCheck *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error) {
	log.FromContext(ctx).Info("trying SPIAccessCheck on common.io. This is not supported yet.")
	return &api.SPIAccessCheckStatus{
		Accessibility: api.SPIAccessCheckAccessibilityUnknown,
		ErrorReason:   api.SPIAccessCheckErrorNotImplemented,
		ErrorMessage:  "Access check for common.io is not implemented.",
	}, nil
}

func (g *Common) MapToken(_ context.Context, _ *api.SPIAccessTokenBinding, token *api.SPIAccessToken, tokenData *api.Token) (serviceprovider.AccessTokenMapper, error) {
	return serviceprovider.DefaultMapToken(token, tokenData)
}

func (g *Common) Validate(ctx context.Context, _ serviceprovider.Validated) (serviceprovider.ValidationResult, error) {
	return serviceprovider.ValidationResult{}, nil
}
