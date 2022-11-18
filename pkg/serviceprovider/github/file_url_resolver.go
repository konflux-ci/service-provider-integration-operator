package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/google/go-github/v45/github"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type fileUrlResolver struct {
	httpClient      *http.Client
	ghClientBuilder githubClientBuilder
}

var _ serviceprovider.FileUrlResolver = (*fileUrlResolver)(nil)

var (
	unexpectedStatusCodeError  = errors.New("unexpected status code from GitHub API")
	fileSizeLimitExceededError = errors.New("failed to retrieve file: size too big")
	pathIsADirectoryError      = errors.New("provided path refers to a directory, not file")
)

var GithubURLRegexp = regexp.MustCompile(`(?Um)^(?:https)(?:\:\/\/)github.com/(?P<owner>[^/]+)/(?P<repo>[^/]+)(.git)?$`)
var GithubURLRegexpNames = GithubURLRegexp.SubexpNames()
var maxFileSizeLimit int = 2097152

func (f fileUrlResolver) Resolve(ctx context.Context, repoUrl, filepath, ref string, token *api.SPIAccessToken) (string, error) {
	result := GithubURLRegexp.FindAllStringSubmatch(repoUrl, -1)
	m := map[string]string{}
	for i, n := range result[0] {
		m[GithubURLRegexpNames[i]] = n
	}
	lg := log.FromContext(ctx)
	ghClient, err := f.ghClientBuilder.createAuthenticatedGhClient(ctx, token)
	file, dir, resp, err := ghClient.Repositories.GetContents(ctx, m["owner"], m["repo"], filepath, &github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		bytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%w: %d. Response: %s", unexpectedStatusCodeError, resp.StatusCode, string(bytes))
	}
	if file == nil && dir != nil {
		return "", fmt.Errorf("%w: %s", pathIsADirectoryError, filepath)
	}
	if file.GetSize() > maxFileSizeLimit {
		lg.Error(err, "file size too big")
		return "", fmt.Errorf("%w: (%d)", fileSizeLimitExceededError, file.Size)
	}
	return file.GetDownloadURL(), nil
}
