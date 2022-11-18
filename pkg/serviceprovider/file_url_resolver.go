package serviceprovider

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type FileUrlResolver interface {

	// Resolve will check whether the provided repository URL matches a known SCM pattern,
	// and resolves input params into valid file download URL.
	// Params are repository, path to the file inside the repository, Git reference (branch/tag/commitId)
	// and SPIAccessToken instance to help with authentication
	Resolve(ctc context.Context, repoUrl, filepath, ref string, token *api.SPIAccessToken) (string, error)
}
