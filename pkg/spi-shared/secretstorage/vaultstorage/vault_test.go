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

package vaultstorage

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	prometheusTest "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
	"github.com/stretchr/testify/assert"
)

func TestMetricCollection(t *testing.T) {
	ctx := context.Background()
	cluster, storage, _, _ := CreateTestVaultSecretStorageWithAuthAndMetrics(t, prometheus.NewPedanticRegistry())
	assert.NoError(t, storage.Initialize(ctx))
	defer cluster.Cleanup()

	_, err := storage.Get(ctx, secretstorage.SecretID{})
	assert.ErrorIs(t, err, secretstorage.NotFoundError)

	assert.Greater(t, prometheusTest.CollectAndCount(vaultRequestCountMetric), 0)
	assert.Greater(t, prometheusTest.CollectAndCount(vaultResponseTimeMetric), 0)
}
