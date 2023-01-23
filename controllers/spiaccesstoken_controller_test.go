package controllers

import (
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEnsureLabels(t *testing.T) {
	t.Run("sets the predefined", func(t *testing.T) {
		at := api.SPIAccessToken{
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "https://hello",
			},
		}

		assert.True(t, ensureLabels(&at, "sp_type"))
		assert.Equal(t, "sp_type", at.Labels[api.ServiceProviderTypeLabel])
		assert.Equal(t, "hello", at.Labels[api.ServiceProviderHostLabel])
	})

	t.Run("doesn't overwrite existing", func(t *testing.T) {
		at := api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"a":                          "av",
					"b":                          "bv",
					api.ServiceProviderHostLabel: "orig-host",
				},
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "https://hello",
			},
		}

		assert.True(t, ensureLabels(&at, "sp_type"))
		assert.Equal(t, "sp_type", at.Labels[api.ServiceProviderTypeLabel])
		assert.Equal(t, "hello", at.Labels[api.ServiceProviderHostLabel])
		assert.Equal(t, "av", at.Labels["a"])
		assert.Equal(t, "bv", at.Labels["b"])
	})
}
