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

package oauth

import (
	"fmt"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	authz "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AuthenticatingClient is just a typedef that advertises that it is safe to use the WithAuthIntoContext or
// WithAuthFromRequestIntoContext functions with clients having this type.
type AuthenticatingClient client.Client

// CreateClient creates a new client based on the provided configuration. Note that configuration is potentially
// modified during the call.
func CreateClient(cfg *rest.Config, options client.Options) (AuthenticatingClient, error) {
	var err error
	scheme := options.Scheme
	if scheme == nil {
		scheme = runtime.NewScheme()
		options.Scheme = scheme
	}

	if err = corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add corev1 to scheme: %w", err)
	}

	if err = v1beta1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add api to the scheme: %w", err)
	}

	if err = authz.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add authz to the scheme: %w", err)
	}

	AugmentConfiguration(cfg)
	cl, err := client.New(cfg, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create a kubernetes client: %w", err)
	}

	return cl, nil
}
