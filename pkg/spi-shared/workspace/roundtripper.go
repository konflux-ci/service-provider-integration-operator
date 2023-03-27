package workspace

import (
	"fmt"
	"net/http"
	"path"
)

type RoundTripper struct {
	Next http.RoundTripper
}

func (w RoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if wsName, hasWsName := FromContext(request.Context()); hasWsName {
		request.URL.Path = path.Join("/workspaces", wsName, request.URL.Path)
	}
	response, err := w.Next.RoundTrip(request)
	if err != nil {
		return nil, fmt.Errorf("failed to run next http roundtrip: %w", err)
	}
	return response, nil
}
