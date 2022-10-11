// Copyright (c) 2021 - 2022 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitfile

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TokenFetcher is the interface for the authentication token suppliers which are provides tokens as a AuthHeaderStruct
// instances
type TokenFetcher interface {
	BuildAuthHeader(ctx context.Context, namespace, secretName string) (map[string]string, error)
}

func buildAuthHeader(ctx context.Context, cl client.Client, repoUrl, namespace, secretName string) (map[string]string, error) {
	fetcher := &SpiTokenFetcher{
		k8sClient: cl,
	}
	headerMap, err := fetcher.BuildAuthHeader(ctx, namespace, secretName)
	if err != nil {
		log.FromContext(ctx).Error(err, "Unable to prepare authentication header for repository", "repoUrl", repoUrl)
		return nil, fmt.Errorf("fetcher failed to build the auth header: %w", err)
	}
	return headerMap, nil
}
