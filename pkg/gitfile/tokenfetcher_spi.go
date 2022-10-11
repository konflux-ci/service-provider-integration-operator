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

	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SpiTokenFetcher token fetcher implementation that looks for token in the specific ENV variable.
type SpiTokenFetcher struct {
	k8sClient client.Client
}

var secretDataEmptyError = stderrors.New("error reading the secret: data is empty")

func (s *SpiTokenFetcher) BuildAuthHeader(ctx context.Context, namespace, secretName string) (map[string]string, error) {

	lg := log.FromContext(ctx)
	lg.Info("Reading the credentials secret", "secret", secretName)
	// reading token secret
	tokenSecret := &corev1.Secret{}
	err := s.k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, tokenSecret)
	if err != nil {
		lg.Error(err, "Error reading Token Secret")
		return nil, fmt.Errorf("failed to read the token secret: %w", err)
	}
	if len(tokenSecret.Data) > 0 {
		return map[string]string{"Authorization": "Bearer " + string(tokenSecret.Data["password"])}, nil
	}
	return nil, secretDataEmptyError
}
