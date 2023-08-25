package serviceprovider

import (
	"context"
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
			Name:      "matching-name",
			Namespace: "default",
			Labels: map[string]string{
				api.RSServiceProviderHostLabel: "test",
			},
		},
	}
	matchingJustLabel := &v1beta1.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching-label",
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
			RepoUrl: "https://test",
		},
	}

	cl := mockK8sClient(matchingLabelAndName, matchingJustLabel, nonMatching)

	rsSources := RemoteSecretCredentialsSource{
		RemoteSecretFilter: RemoteSecretFilterFunc(func(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool {
			return remoteSecret.Name == "matching-name"
		}),
		RepoHostParser: RepoUrlParserFromSchemalessUrl,
	}

	remoteSecrets, err := rsSources.LookupCredentialsSource(context.TODO(), cl, &check)
	assert.NoError(t, err)
	assert.Len(t, remoteSecrets, 1)
	assert.Equal(t, "matching-name", remoteSecrets.Name)
}

func TestRSSource_LookupCredentials(t *testing.T) {
	//remoteSecrets := []v1beta1.RemoteSecret{{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name:      "matching-rs",
	//		Namespace: "default",
	//		Annotations: map[string]string{
	//			api.RSServiceProviderRepositoryAnnotation: "test/repo,diff/repo",
	//		},
	//	},
	//	Status: v1beta1.RemoteSecretStatus{
	//		Targets: []v1beta1.TargetStatus{{
	//			Namespace:  "default",
	//			SecretName: "rs-secret",
	//		}},
	//	},
	//}, {
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name:      "non-matching",
	//		Namespace: "default",
	//		Annotations: map[string]string{
	//			api.RSServiceProviderRepositoryAnnotation: "diff/repo,anoth/repo",
	//		},
	//	},
	//}, {
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name:      "non-matching",
	//		Namespace: "default",
	//	},
	//}}

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

	cl := mockK8sClient(&remoteSecretSecret)
	rsSource := RemoteSecretCredentialsSource{
		RemoteSecretFilter: RemoteSecretFilterFunc(func(ctx context.Context, matchable Matchable, remoteSecret *v1beta1.RemoteSecret) bool {
			return remoteSecret.Name == "matching-name"
		}),
		RepoHostParser: RepoUrlParserFromSchemalessUrl,
	}

	rs, err := rsSource.LookupCredentials(context.TODO(), cl, &check)
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	//assert.NotNil(t, secret)
	//assert.Equal(t, "matching-rs", rs.Name)
	//assert.Equal(t, "rs-secret", secret.Name)
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
