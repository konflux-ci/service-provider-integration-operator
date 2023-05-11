package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteSecretValidate(t *testing.T) {
	t.Run("accepts single target", func(t *testing.T) {
		rs := &RemoteSecret{
			Spec: RemoteSecretSpec{
				Targets: []RemoteSecretTarget{
					{
						Namespace: "ns-1",
					},
				},
			},
		}

		assert.NoError(t, rs.Validate())
	})

	t.Run("accepts multiple targets in differents namespaces", func(t *testing.T) {
		rs := &RemoteSecret{
			Spec: RemoteSecretSpec{
				Targets: []RemoteSecretTarget{
					{
						Namespace: "ns-1",
					},
					{
						Namespace: "ns-2",
					},
					{
						Namespace: "ns-3",
					},
				},
			},
		}

		assert.NoError(t, rs.Validate())
	})

	t.Run("rejects multiple targets in single namespace", func(t *testing.T) {
		rs := &RemoteSecret{
			Spec: RemoteSecretSpec{
				Targets: []RemoteSecretTarget{
					{
						Namespace: "ns-1",
					},
					{
						Namespace: "ns-1",
					},
				},
			},
		}

		err := rs.Validate()

		assert.Error(t, err)
		assert.Equal(t, "a single namespace targetted from multiple targets: targets on indices 0 and 1 point to the same namespace ns-1", err.Error())
	})

}
