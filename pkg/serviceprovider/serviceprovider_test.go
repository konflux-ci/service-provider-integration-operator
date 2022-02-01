package serviceprovider

import (
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	integrationtests "github.com/redhat-appstudio/service-provider-integration-operator/integration_tests"
	"github.com/stretchr/testify/assert"
)

func TestGetAllScopesUniqueValues(t *testing.T) {
	sp := integrationtests.TestServiceProvider{
		TranslateToScopesImpl: func(permission api.Permission) []string {
			return []string{string(permission.Type), string(permission.Area)}
		},
	}
	perms := &api.Permissions{
		Required: []api.Permission{
			{
				Type: "a",
				Area: "b",
			},
			{
				Type: "a",
				Area: "c",
			},
		},
		AdditionalScopes: []string{"a", "b", "d", "e"},
	}

	scopes := GetAllScopes(sp, perms)

	expected := []string{"a", "b", "c", "d", "e"}
	for _, e := range expected {
		assert.Contains(t, scopes, e)
	}
	assert.Len(t, scopes, len(expected))
}
