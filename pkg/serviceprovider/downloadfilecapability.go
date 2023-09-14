//
// Copyright (c) 2021 Red Hat, Inc.
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

package serviceprovider

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type FileDownloadNotSupportedError struct {
}

func (f FileDownloadNotSupportedError) Error() string {
	return "provided repository URL does not supports file downloading"
}

type NoCredentialsFoundError struct{}

func (f NoCredentialsFoundError) Error() string {
	return "no suitable credentials found for the SPIFileContentRequest"
}

// DownloadFileCapability indicates an ability of given SCM provider to download files from repository.
type DownloadFileCapability interface {
	DownloadFile(ctx context.Context, client client.Client, request *api.SPIFileContentRequest, maxFileSizeLimit int) (string, error)
}
