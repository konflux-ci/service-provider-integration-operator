package tokenstorage

import (
	"context"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testToken = &v1beta1.Token{
	AccessToken:  "testAccessToken",
	TokenType:    "testTokenType",
	RefreshToken: "testRefreshToken",
	Expiry:       123,
}

var testSpiAccessToken = &v1beta1.SPIAccessToken{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "testSpiAccessToken",
		Namespace: "testNamespace",
	},
}

func TestStoreAndGet(t *testing.T) {
	cluster, storage := CreateTestVaultTokenStorage(t)
	defer cluster.Cleanup()

	stored, err := storage.Store(context.TODO(), testSpiAccessToken, testToken)
	assert.NoError(t, err)
	assert.NotEmpty(t, stored)

	gettedToken, err := storage.Get(context.TODO(), testSpiAccessToken)
	assert.NoError(t, err)
	assert.NotNil(t, gettedToken)
	assert.EqualValues(t, testToken, gettedToken)

	err = storage.Delete(context.TODO(), testSpiAccessToken)
	assert.NoError(t, err)

	gettedToken, err = storage.Get(context.TODO(), testSpiAccessToken)
	assert.NoError(t, err)
	assert.Nil(t, gettedToken)
}
