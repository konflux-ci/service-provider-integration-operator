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

package secretstorage

import (
	"context"
	"fmt"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

func TestDefaultTypedSecretStorage_Store(t *testing.T) {
	record := testStorageCall(func(dtss *DefaultTypedSecretStorage[bool, bool]) {
		dtss.Store(context.TODO(), pointer.Bool(true), nil)
	}, CallsRecord[bool]{})

	assert.True(t, record.ToIDCalled)
	assert.True(t, record.SerializeCalled)
	assert.True(t, record.StoreCalled)

	assert.False(t, record.DeserializeCalled)
	assert.False(t, record.GetCalled)
	assert.False(t, record.DeleteCalled)
	assert.False(t, record.InitializeCalled)
}

func TestFailedToId_Store(t *testing.T) {
	record := testStorageCall(func(dtss *DefaultTypedSecretStorage[int, bool]) {
		dtss.Store(context.TODO(), pointer.Int(42), nil)
	}, CallsRecord[int]{ToIDFunc: func(i *int) (*SecretID, error) { return nil, fmt.Errorf("err") }})

	assert.True(t, record.ToIDCalled)

	assert.False(t, record.DeleteCalled)
	assert.False(t, record.GetCalled)
	assert.False(t, record.DeserializeCalled)
	assert.False(t, record.StoreCalled)
	assert.False(t, record.SerializeCalled)
	assert.False(t, record.InitializeCalled)
}

func TestDefaultTypedSecretStorage_Get(t *testing.T) {
	record := testStorageCall(func(dtss *DefaultTypedSecretStorage[int, bool]) {
		dtss.Get(context.TODO(), pointer.Int(42))
	}, CallsRecord[int]{})

	assert.True(t, record.ToIDCalled)
	assert.True(t, record.GetCalled)
	assert.True(t, record.DeserializeCalled)

	assert.False(t, record.StoreCalled)
	assert.False(t, record.SerializeCalled)
	assert.False(t, record.DeleteCalled)
	assert.False(t, record.InitializeCalled)
}

func TestFailedToId_Get(t *testing.T) {
	record := testStorageCall(func(dtss *DefaultTypedSecretStorage[int, bool]) {
		dtss.Get(context.TODO(), pointer.Int(42))
	}, CallsRecord[int]{ToIDFunc: func(i *int) (*SecretID, error) { return nil, fmt.Errorf("err") }})

	assert.True(t, record.ToIDCalled)

	assert.False(t, record.DeleteCalled)
	assert.False(t, record.GetCalled)
	assert.False(t, record.DeserializeCalled)
	assert.False(t, record.StoreCalled)
	assert.False(t, record.SerializeCalled)
	assert.False(t, record.InitializeCalled)
}

func TestDefaultTypedSecretStorage_Delete(t *testing.T) {
	record := testStorageCall(func(dtss *DefaultTypedSecretStorage[int, bool]) {
		dtss.Delete(context.TODO(), pointer.Int(42))
	}, CallsRecord[int]{})

	assert.True(t, record.ToIDCalled)
	assert.True(t, record.DeleteCalled)

	assert.False(t, record.GetCalled)
	assert.False(t, record.DeserializeCalled)
	assert.False(t, record.StoreCalled)
	assert.False(t, record.SerializeCalled)
	assert.False(t, record.InitializeCalled)
}

func TestFailedToId_Delete(t *testing.T) {
	record := testStorageCall(func(dtss *DefaultTypedSecretStorage[int, bool]) {
		dtss.Delete(context.TODO(), pointer.Int(42))
	}, CallsRecord[int]{ToIDFunc: func(i *int) (*SecretID, error) { return nil, fmt.Errorf("err") }})

	assert.True(t, record.ToIDCalled)

	assert.False(t, record.DeleteCalled)
	assert.False(t, record.GetCalled)
	assert.False(t, record.DeserializeCalled)
	assert.False(t, record.StoreCalled)
	assert.False(t, record.SerializeCalled)
	assert.False(t, record.InitializeCalled)
}

func TestDefaultTypedSecretStorage_Initialize(t *testing.T) {
	record := testStorageCall(func(dtss *DefaultTypedSecretStorage[int, bool]) {
		dtss.Initialize(context.TODO())
	}, CallsRecord[int]{})

	// this is explicitly a noop, so test that it doesn't meddle
	// with the underlying storage.
	assert.False(t, record.InitializeCalled)

	assert.False(t, record.ToIDCalled)
	assert.False(t, record.DeleteCalled)
	assert.False(t, record.GetCalled)
	assert.False(t, record.DeserializeCalled)
	assert.False(t, record.StoreCalled)
	assert.False(t, record.SerializeCalled)
}

func TestSerializeJSON(t *testing.T) {
	data, err := SerializeJSON(pointer.Bool(true))
	assert.NoError(t, err)
	assert.Equal(t, []byte("true"), data)
}

func TestDeserializeJSON(t *testing.T) {
	data := []byte("42")
	val := pointer.Int(0)
	assert.NoError(t, DeserializeJSON(data, val))
	assert.Equal(t, 42, *val)
}

func TestObjectToId(t *testing.T) {
	token := &api.SPIAccessToken{
		ObjectMeta: v1.ObjectMeta{
			UID:       "test-uid",
			Namespace: "test-ns",
			Name:      "test-n",
		},
	}

	id, err := ObjectToID(token)

	assert.NotNil(t, id)
	assert.Equal(t, id.Name, "test-n")
	assert.Equal(t, id.Namespace, "test-ns")
	assert.Equal(t, id.Uid, types.UID("test-uid"))
	assert.NoError(t, err)
}

func TestObjectToIdNoUid(t *testing.T) {
	token := &api.SPIAccessToken{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-n",
		},
	}

	id, err := ObjectToID(token)

	assert.Nil(t, id)
	assert.Error(t, err)
}

type CallsRecord[K any] struct {
	ToIDCalled        bool
	ToIDFunc          func(i *K) (*SecretID, error)
	SerializeCalled   bool
	DeserializeCalled bool
	StoreCalled       bool
	GetCalled         bool
	DeleteCalled      bool
	InitializeCalled  bool
}

func testStorageCall[K any, D any](test func(*DefaultTypedSecretStorage[K, D]), record CallsRecord[K]) CallsRecord[K] {
	instance := DefaultTypedSecretStorage[K, D]{
		DataTypeName: "kachny",
		SecretStorage: &TestSecretStorage{
			InitializeImpl: func(_ context.Context) error {
				record.InitializeCalled = true
				return nil
			},
			StoreImpl: func(_ context.Context, _ SecretID, _ []byte) error {
				record.StoreCalled = true
				return nil
			},
			GetImpl: func(_ context.Context, _ SecretID) ([]byte, error) {
				record.GetCalled = true
				return nil, nil
			},
			DeleteImpl: func(_ context.Context, _ SecretID) error {
				record.DeleteCalled = true
				return nil
			},
		},
		ToID: func(i *K) (*SecretID, error) {
			record.ToIDCalled = true
			if record.ToIDFunc != nil {
				return record.ToIDFunc(i)
			} else {
				return &SecretID{}, nil
			}
		},
		Serialize: func(d *D) ([]byte, error) {
			record.SerializeCalled = true
			return nil, nil
		},
		Deserialize: func(b []byte, d *D) error {
			record.DeserializeCalled = true
			return nil
		},
	}

	test(&instance)

	return record
}
