package serviceprovider

import (
	"context"
	"testing"
	"time"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSPISource_LookupCredentialsSource(t *testing.T) {
	matchingToken := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching",
			Namespace: "default",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: "test",
				api.ServiceProviderHostLabel: "fake.sp",
			},
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
		},
	}
	nonMatchingToken1 := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-matching1",
			Namespace: "default",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: "test",
				api.ServiceProviderHostLabel: "fake.sp",
			},
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
		},
	}
	nonMatchingToken2 := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-matching2",
			Namespace: "default",
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
		},
	}

	cl := mockK8sClient(matchingToken, nonMatchingToken1, nonMatchingToken2)

	spiSource := mockSPISource(cl)
	tkn, err := spiSource.LookupCredentialsSource(context.TODO(), cl, &api.SPIAccessTokenBinding{
		Spec: api.SPIAccessTokenBindingSpec{
			RepoUrl: "https://fake.sp",
		},
	})
	assert.NoError(t, err)

	assert.Equal(t, "matching", tkn.Name)
}

func TestSPISource_LookupCredentials(t *testing.T) {
	matchingToken := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching",
			Namespace: "default",
			Labels: map[string]string{
				api.ServiceProviderTypeLabel: "test",
				api.ServiceProviderHostLabel: "fake.sp",
			},
		},
		Status: api.SPIAccessTokenStatus{
			Phase: api.SPIAccessTokenPhaseReady,
		},
	}

	cl := mockK8sClient(matchingToken)

	spiSource := mockSPISource(cl)
	spiSource.TokenStorage = tokenstorage.TestTokenStorage{
		GetImpl: func(ctx context.Context, token *api.SPIAccessToken) (*api.Token, error) {
			return &api.Token{
				Username:    "user123",
				AccessToken: "token123",
			}, nil
		},
	}
	cred, err := spiSource.LookupCredentials(context.TODO(), cl, &api.SPIAccessTokenBinding{
		Spec: api.SPIAccessTokenBindingSpec{
			RepoUrl: "https://fake.sp",
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "user123", cred.Username)
	assert.Equal(t, "token123", cred.Password)
}

func mockSPISource(cl client.Client) SPIAccessTokenCredentialsSource {
	cache := MetadataCache{
		Client:                    cl,
		ExpirationPolicy:          &TtlMetadataExpirationPolicy{Ttl: 1 * time.Hour},
		CacheServiceProviderState: true,
	}
	spiSource := SPIAccessTokenCredentialsSource{
		ServiceProviderType: "test",
		TokenFilter: TokenFilterFunc(func(ctx context.Context, binding Matchable, token *api.SPIAccessToken) (bool, error) {
			return token.Name == "matching", nil
		}),
		MetadataProvider: MetadataProviderFunc(func(_ context.Context, _ *api.SPIAccessToken, _ bool) (*api.TokenMetadata, error) {
			return &api.TokenMetadata{
				UserId: "42",
			}, nil
		}),
		MetadataCache:  &cache,
		RepoHostParser: RepoUrlParserFromUrl,
	}
	return spiSource
}
