package controllers

import (
	"testing"

	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRemoteSecretApplicableForEnvironment(t *testing.T) {
	rs := v1beta1.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{EnvironmentLabelAndAnnotationName: "test"},
			Annotations: map[string]string{EnvironmentLabelAndAnnotationName: "prod,stage"},
		},
	}
	t.Run("has label", func(t *testing.T) {
		rs := rs
		rs.Annotations = nil
		assert.True(t, remoteSecretApplicableForEnvironment(rs, "test"))
		assert.False(t, remoteSecretApplicableForEnvironment(rs, "prod"))
	})
	t.Run("has annotation", func(t *testing.T) {
		rs := rs
		rs.Labels = nil
		assert.True(t, remoteSecretApplicableForEnvironment(rs, "prod"))
		assert.False(t, remoteSecretApplicableForEnvironment(rs, "test"))
	})
	t.Run("has both", func(t *testing.T) {
		assert.True(t, remoteSecretApplicableForEnvironment(rs, "prod"))
		assert.False(t, remoteSecretApplicableForEnvironment(rs, "test"))
	})
	t.Run("has none", func(t *testing.T) {
		assert.True(t, remoteSecretApplicableForEnvironment(v1beta1.RemoteSecret{}, "any"))
	})
}
