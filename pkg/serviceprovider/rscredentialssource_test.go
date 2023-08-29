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
	"strings"
	"testing"

	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRSSource_LookupCredentialsSource(t *testing.T) {
	matchingLabelAndName := &v1beta1.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching-with-preference",
			Namespace: "default",
			Labels: map[string]string{
				api.RSServiceProviderHostLabel: "test",
			},
			Annotations: map[string]string{
				api.RSServiceProviderRepositoryAnnotation: "repo/name,other/name",
			},
		},
	}
	matchingJustLabel := &v1beta1.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching-1",
			Namespace: "default",
			Labels: map[string]string{
				api.RSServiceProviderHostLabel: "test",
			},
		},
	}
	nonMatching := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-matching",
			Namespace: "default",
			Labels: map[string]string{
				api.RSServiceProviderHostLabel: "diffrent.host",
			},
		},
	}

	check := api.SPIAccessCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "accessCheck",
			Namespace: "default",
		},
		Spec: api.SPIAccessCheckSpec{
			RepoUrl: "https://test/repo/name",
		},
	}

	cl := mockK8sClient(matchingLabelAndName, matchingJustLabel, nonMatching)

	rsSources := RemoteSecretCredentialsSource{
		RemoteSecretFilter: RemoteSecretFilterFunc(func(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool {
			return strings.HasPrefix(remoteSecret.Name, "matching")
		}),
		RepoHostParser: RepoUrlParserFromSchemalessUrl,
	}

	remoteSecret, err := rsSources.LookupCredentialsSource(context.TODO(), cl, &check)
	assert.NoError(t, err)
	assert.NotNil(t, remoteSecret)
	assert.Equal(t, "matching-with-preference", remoteSecret.Name)
}

func TestRSSource_LookupCredentials(t *testing.T) {
	remoteSecret := v1beta1.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching-rs",
			Namespace: "default",
			Labels: map[string]string{
				api.RSServiceProviderHostLabel: "test",
			},
		},
		Status: v1beta1.RemoteSecretStatus{
			Targets: []v1beta1.TargetStatus{{
				Namespace:  "default",
				SecretName: "rs-secret",
			}},
		},
	}

	check := api.SPIAccessCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "accessCheck",
			Namespace: "default",
		},
		Spec: api.SPIAccessCheckSpec{
			RepoUrl: "https://test",
		},
	}

	remoteSecretSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rs-secret",
			Namespace: "default",
		},
	}

	cl := mockK8sClient(&remoteSecret, &remoteSecretSecret)
	rsSource := RemoteSecretCredentialsSource{
		RemoteSecretFilter: RemoteSecretFilterFunc(func(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool {
			return remoteSecret.Name == "matching-rs"
		}),
		RepoHostParser: RepoUrlParserFromSchemalessUrl,
	}

	cred, err := rsSource.LookupCredentials(context.TODO(), cl, &check)
	assert.NoError(t, err)
	assert.NotNil(t, cred)
	assert.Equal(t, "matching-rs", cred.SourceObjectName)
}

func TestGetLocalNamespaceTargetIndex(t *testing.T) {
	targets := []v1beta1.TargetStatus{
		{
			Namespace:  "ns",
			ApiUrl:     "url",
			SecretName: "sec0",
		},
		{
			Namespace:  "ns",
			SecretName: "sec1",
			Error:      "some error",
		},
		{
			Namespace:  "diff-ns",
			SecretName: "sec2",
		},
	}

	assert.Equal(t, -1, getLocalNamespaceTargetIndex(targets, "ns"))
	targets = append(targets, v1beta1.TargetStatus{
		Namespace:  "ns",
		SecretName: "sec3",
	})
	assert.Equal(t, 3, getLocalNamespaceTargetIndex(targets, "ns"))
}

func mockK8sClient(objects ...client.Object) client.WithWatch {
	sch := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(sch))
	utilruntime.Must(api.AddToScheme(sch))
	utilruntime.Must(v1beta1.AddToScheme(sch))
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(objects...).Build()
}
