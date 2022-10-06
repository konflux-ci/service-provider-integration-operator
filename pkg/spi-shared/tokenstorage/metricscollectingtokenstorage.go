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

package tokenstorage

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/metrics"
)

var (
	accessHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "redhat_appstudio",
		Subsystem: "spi",
		Name:      "vault_access_duration_seconds",
		Help:      "The time to complete the requests to Vault",
		Buckets:   []float64{0.05, 0.1, 0.3, 0.5, 0.7, 1},
	}, []string{"access_type", "is_err"})

	// Because we have only a handful of metrics here, we can afford to pre-create all the combinations. This is more
	// performant than using WithLabelValues every time.

	storeHist                                    = accessHist.WithLabelValues("store", "false")
	storeErrorHist                               = accessHist.WithLabelValues("store", "true")
	storeObserver  metrics.ValueObserver1[error] = metrics.ValueObserverFunc1[error](func(err error, secs float64) {
		recordAccess(storeHist, storeErrorHist, err, secs)
	})

	getHist                                                = accessHist.WithLabelValues("get", "false")
	getErrorHist                                           = accessHist.WithLabelValues("get", "true")
	getObserver  metrics.ValueObserver2[*api.Token, error] = metrics.ValueObserverFunc2[*api.Token, error](func(_ *api.Token, err error, secs float64) {
		recordAccess(getHist, getErrorHist, err, secs)
	})

	deleteHist                                    = accessHist.WithLabelValues("delete", "false")
	deleteErrorHist                               = accessHist.WithLabelValues("delete", "true")
	deleteObserver  metrics.ValueObserver1[error] = metrics.ValueObserverFunc1[error](func(err error, secs float64) {
		recordAccess(deleteHist, deleteErrorHist, err, secs)
	})
)

var _ TokenStorage = (*MetricsCollectingTokenStorage)(nil)

type MetricsCollectingTokenStorage struct {
	MetricsRegisterer prometheus.Registerer
	TokenStorage      TokenStorage
}

func (t *MetricsCollectingTokenStorage) Initialize(ctx context.Context) error {
	if err := t.MetricsRegisterer.Register(accessHist); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	if err := t.TokenStorage.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize the wrapped token storage: %w", err)
	}

	return nil
}

func (t *MetricsCollectingTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	timer := metrics.NewValueTimer1(storeObserver)

	//nolint:wrapcheck // this is just propagating our error
	return timer.ObserveValuesAndDuration(t.TokenStorage.Store(ctx, owner, token))
}

func (t *MetricsCollectingTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	timer := metrics.NewValueTimer2(getObserver)

	//nolint:wrapcheck // this is just propagating our error
	return timer.ObserveValuesAndDuration(t.TokenStorage.Get(ctx, owner))
}

func (t *MetricsCollectingTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	timer := metrics.NewValueTimer1(deleteObserver)

	//nolint:wrapcheck // this is just propagating our error
	return timer.ObserveValuesAndDuration(t.TokenStorage.Delete(ctx, owner))
}

func recordAccess(successHist prometheus.Observer, errorHist prometheus.Observer, err error, secs float64) {
	h := successHist
	if err != nil {
		h = errorHist
	}
	h.Observe(secs)
}
