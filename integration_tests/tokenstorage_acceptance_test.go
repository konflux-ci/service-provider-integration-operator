package integrationtests

import (
	"context"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage/vaultstorage"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

// Testsuite that runs same tests against multiple tokenstorage implementations.
// Tests cares just about TokenStorage interface and makes sure that implementations behaves the same.
// Runs against real storages, see comments on Test* functions for more details.

// TestVault runs against local in-memory Vault. No external dependency or setup is needed, test runs everytime.
func TestVault(t *testing.T) {
	cluster, storage := vaultstorage.CreateTestVaultTokenStorage(t)
	assert.NoError(t, storage.Initialize(context.Background()))
	defer cluster.Cleanup()

	testStorage(t, context.TODO(), storage)
}

func testStorage(t *testing.T, ctx context.Context, storage tokenstorage.TokenStorage) {
	refreshTestData()

	// get non-existing
	gettedToken, err := storage.Get(ctx, testSpiAccessToken)
	assert.NoError(t, err)
	assert.Nil(t, gettedToken)

	// delete non-existing
	err = storage.Delete(ctx, testSpiAccessToken)
	assert.NoError(t, err)

	// create
	err = storage.Store(ctx, testSpiAccessToken, testToken)
	assert.NoError(t, err)

	// write, let's wait a bit
	time.Sleep(1 * time.Second)

	// get
	gettedToken, err = storage.Get(ctx, testSpiAccessToken)
	assert.NoError(t, err)
	assert.NotNil(t, gettedToken)
	assert.EqualValues(t, testToken, gettedToken)

	// update
	testToken.AccessToken += "-update"
	err = storage.Store(ctx, testSpiAccessToken, testToken)
	assert.NoError(t, err)

	// write, let's wait a bit
	time.Sleep(1 * time.Second)

	// get
	gettedToken, err = storage.Get(ctx, testSpiAccessToken)
	assert.NoError(t, err)
	assert.NotNil(t, gettedToken)
	assert.EqualValues(t, testToken, gettedToken)

	// delete
	err = storage.Delete(ctx, testSpiAccessToken)
	assert.NoError(t, err)

	// write, let's wait a bit
	time.Sleep(1 * time.Second)

	// get deleted
	gettedToken, err = storage.Get(ctx, testSpiAccessToken)
	assert.NoError(t, err)
	assert.Nil(t, gettedToken)

	// if for some reason update failed, we try to delete secret before update so we don't leave mess
	testToken.AccessToken = strings.TrimSuffix(testToken.AccessToken, "-update")
	err = storage.Delete(ctx, testSpiAccessToken)
	assert.NoError(t, err)
}

var (
	testToken          *v1beta1.Token
	testSpiAccessToken *v1beta1.SPIAccessToken
)

func refreshTestData() {
	random, _, _ := strings.Cut(string(uuid.NewUUID()), "-")
	testToken = &v1beta1.Token{
		Username:     "testUsername-" + random,
		AccessToken:  "testAccessToken-" + random,
		TokenType:    "testTokenType-" + random,
		RefreshToken: "testRefreshToken-" + random,
		Expiry:       rand.Uint64() % 1000,
	}

	testSpiAccessToken = &v1beta1.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testSpiAccessToken-" + random,
			Namespace: "testNamespace-" + random,
		},
	}
}
