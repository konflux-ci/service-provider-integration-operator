package gitlab

import (
	"context"
	"errors"
	"fmt"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/xanzy/go-gitlab"
	"io"
	"net/http"
	"regexp"
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
	var opts *gitlab.GetFileOptions = nil
	if ref != "" {
		opts = &gitlab.GetFileOptions{Ref: &ref}
	}
	file, resp, err := glClient.RepositoryFiles.GetFile(m["owner"]+"/"+m["project"], filepath, opts)
	if err != nil {
		bytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%w: %d. Response: %s", unexpectedStatusCodeError, resp.StatusCode, string(bytes))
	}

	if file.Size > maxFileSizeLimit {
		lg.Error(err, "file size too big")
		return "", fmt.Errorf("%w: (%d)", fileSizeLimitExceededError, file.Size)
	}
	return fmt.Sprintf(
		f.baseUrl+"/projects/%s/repository/files/%s/raw?ref=%s",
		gitlab.PathEscape(m["owner"]+"/"+m["project"]),
		gitlab.PathEscape(filepath),
		ref,
	), nil
}
