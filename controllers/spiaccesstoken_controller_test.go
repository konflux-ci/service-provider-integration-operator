package controllers

import (
	"context"
	"testing"

	"github.com/kcp-dev/logicalcluster/v2"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateUploadUrl(t *testing.T) {
	r := &SPIAccessTokenReconciler{
		Configuration: opconfig.OperatorConfiguration{
			SharedConfiguration: config.SharedConfiguration{
				BaseUrl: "blabol",
			},
		},
	}

	at := &api.SPIAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-atname",
		},
	}

	t.Run("kcp-env", func(t *testing.T) {
		ctx := logicalcluster.WithCluster(context.TODO(), logicalcluster.New("workspace"))
		url := r.createUploadUrl(ctx, at)
		assert.Contains(t, url, "blabol")
		assert.Contains(t, url, "workspace")
		assert.Contains(t, url, "test-namespace")
		assert.Contains(t, url, "test-atname")
	})

	t.Run("non-kcp-env", func(t *testing.T) {
		url := r.createUploadUrl(context.TODO(), at)
		assert.Contains(t, url, "blabol")
		assert.NotContains(t, url, "workspace")
		assert.Contains(t, url, "test-namespace")
		assert.Contains(t, url, "test-atname")
	})
}
