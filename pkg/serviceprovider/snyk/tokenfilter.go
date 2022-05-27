package snyk

import (
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
)

type tokenFilter struct{}

var _ serviceprovider.TokenFilter = (*tokenFilter)(nil)

func (t *tokenFilter) Matches(matchable serviceprovider.Matchable, token *api.SPIAccessToken) (bool, error) {
	//TODO: implement token matching
	return true, nil
}
