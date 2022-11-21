package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/xanzy/go-gitlab"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type fileUrlResolver struct {
	httpClient      *http.Client
	glClientBuilder gitlabClientBuilder
	baseUrl         string
	gitLabUrlRegexp *regexp.Regexp
}

func NewGitlabFileUrlResolver(httpClient *http.Client, glClientBuilder gitlabClientBuilder, baseUrl string) fileUrlResolver {
	return fileUrlResolver{
		httpClient:      httpClient,
		glClientBuilder: glClientBuilder,
		baseUrl:         baseUrl,
		gitLabUrlRegexp: regexp.MustCompile(`(?Um)^` + baseUrl + `/(?P<owner>[^/]+)/(?P<project>[^/]+)(.git)?$`),
	}
}

var _ serviceprovider.FileUrlResolver = (*fileUrlResolver)(nil)

var (
	unexpectedStatusCodeError  = errors.New("unexpected status code from GitLab API")
	fileSizeLimitExceededError = errors.New("failed to retrieve file: size too big")
)

var maxFileSizeLimit int = 2097152

func (f fileUrlResolver) Resolve(ctx context.Context, repoUrl, filepath, ref string, token *api.SPIAccessToken) (string, error) {
	gitLabURLRegexpNames := f.gitLabUrlRegexp.SubexpNames()
	result := f.gitLabUrlRegexp.FindAllStringSubmatch(repoUrl, -1)
	m := map[string]string{}
	for i, n := range result[0] {
		m[gitLabURLRegexpNames[i]] = n
	}
	lg := log.FromContext(ctx)
	glClient, err := f.glClientBuilder.createGitlabAuthClient(ctx, token, f.baseUrl)
	if err != nil {
		return "", fmt.Errorf("failed to create authenticated GitLab client: %w", err)
	}

	var refOption gitlab.GetFileMetaDataOptions

	//ref is required, need to set ir retrieve it
	if ref != "" {
		refOption = gitlab.GetFileMetaDataOptions{Ref: gitlab.String(ref)}
	} else {
		refOption = gitlab.GetFileMetaDataOptions{Ref: gitlab.String("HEAD")}
	}

	file, resp, err := glClient.RepositoryFiles.GetFileMetaData(m["owner"]+"/"+m["project"], filepath, &refOption)
	if err != nil {
		// unfortunately, GitLab library closes the response body, so it is cannot be read
		return "", fmt.Errorf("%w: %d", unexpectedStatusCodeError, resp.StatusCode)
	}

	if file.Size > maxFileSizeLimit {
		lg.Error(err, "file size too big")
		return "", fmt.Errorf("%w: (%d)", fileSizeLimitExceededError, file.Size)
	}
	return fmt.Sprintf(
		f.baseUrl+"/api/v4/projects/%s/repository/files/%s/raw?ref=%s",
		gitlab.PathEscape(m["owner"]+"/"+m["project"]),
		gitlab.PathEscape(filepath),
		*refOption.Ref,
	), nil
}
