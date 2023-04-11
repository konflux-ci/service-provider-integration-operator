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

package clientfactory

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UserAuthK8sClientFactory is produces instances of the client which works on behalf of the user (i.e. using his token for authentication)
// but it is not the workspace-aware. It is used in workspace-less environments like minikube or integration tests etc
type UserAuthK8sClientFactory struct {
	ClientOptions *client.Options
	RestConfig    *rest.Config
}

func (u UserAuthK8sClientFactory) CreateClient(_ context.Context) (client.Client, error) {
	cl, err := client.New(u.RestConfig, *u.ClientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create a user auth kubernetesclient client: %w", err)
	}
	return cl, nil
}
