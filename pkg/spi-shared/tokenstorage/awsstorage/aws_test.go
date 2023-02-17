package awsstorage

import (
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateSecretName(t *testing.T) {
	secretName := generateAwsSecretName(&v1beta1.SPIAccessToken{ObjectMeta: v1.ObjectMeta{Namespace: "tokennamespace", Name: "tokenname"}})

	assert.NotNil(t, secretName)
	assert.Contains(t, *secretName, "tokennamespace")
	assert.Contains(t, *secretName, "tokenname")
}
