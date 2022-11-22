package serviceprovider

import (
	"context"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type FileDownloadNotSupportedError struct {
}

func (f FileDownloadNotSupportedError) Error() string {
	return "provided repository URL does not supports file downloading"
}

//DownloadFileCapability indicates an ability of given SCM provider to download files from repository.
type DownloadFileCapability interface {
	DownloadFile(ctx context.Context, repoUrl, filepath, ref string, token *api.SPIAccessToken) (string, error)
}
