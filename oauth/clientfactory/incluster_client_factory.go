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

// InClusterK8sClientFactory produces instances of the clients that works on behalf of SPI SA;
// (it's quite ok to get just one instance of the client from that factory and pass it to all the consumers)
type InClusterK8sClientFactory struct {
	ClientOptions *client.Options
	RestConfig    *rest.Config
}

func (i InClusterK8sClientFactory) CreateClient(_ context.Context) (client.Client, error) {
	cl, err := client.New(i.RestConfig, *i.ClientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create a in-cluster kubernetesclient client: %w", err)
	}
	return cl, nil
}
