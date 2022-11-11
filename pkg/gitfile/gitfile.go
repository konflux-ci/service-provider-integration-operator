// Copyright (c) 2022 Red Hat, Inc.
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
	stderrors "errors"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var secretDataEmptyError = stderrors.New("error reading the secret: data is empty")

// GetFileContents is a function allowing to retrieve file content from the SCM provider.
func GetFileContents(ctx context.Context, k8sClient client.Client, httpClient http.Client, fileUrl string, namespace, secret string) (io.ReadCloser, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", fileUrl, nil)
	authHeaders, err := buildAuthHeader(ctx, k8sClient, namespace, secret)
	if err != nil {
		return nil, err
	}
	for k, v := range authHeaders {
		req.Header.Add(k, v)
	}
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error reading file content: %w", err)
	}
	return response.Body, nil
}

//buildAuthHeader builds auth headers map from given secret
func buildAuthHeader(ctx context.Context, k8sClient client.Client, namespace, secretName string) (map[string]string, error) {
	lg := log.FromContext(ctx)
	lg.Info("Reading the credentials secret", "secret", secretName)
	// reading token secret
	tokenSecret := &corev1.Secret{}
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, tokenSecret)
	if err != nil {
		lg.Error(err, "Error reading Token Secret")
		return nil, fmt.Errorf("failed to read the token secret: %w", err)
	}
	if len(tokenSecret.Data) > 0 {
		return map[string]string{"Authorization": "Bearer " + string(tokenSecret.Data["password"])}, nil
	}
	return nil, secretDataEmptyError
}
