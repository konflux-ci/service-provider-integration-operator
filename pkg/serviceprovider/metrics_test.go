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
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	prometheusTest "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

func TestRegisterMetrics(t *testing.T) {
	registry := prometheus.NewPedanticRegistry()
	RequestCountMetric.Reset()
	ResponseTimeMetric.Reset()

	assert.NoError(t, RegisterCommonMetrics(registry))

	RequestCountMetric.WithLabelValues("sp", "here.there", "GET", "false", "op").Inc()
	ResponseTimeMetric.WithLabelValues("sp", "here.there", "GET", "200", "op").Observe(42)

	count, err := prometheusTest.GatherAndCount(registry, "redhat_appstudio_spi_service_provider_request_count_total", "redhat_appstudio_spi_service_provider_response_time_seconds")
	assert.Equal(t, 2, count)
	assert.NoError(t, err)
}

func TestRequestMetricsConfig(t *testing.T) {
	test := func(t *testing.T, r util.FakeRoundTrip, expectedLabels map[string]string) {
		registry := prometheus.NewPedanticRegistry()
		RequestCountMetric.Reset()
		ResponseTimeMetric.Reset()

		assert.NoError(t, RegisterCommonMetrics(registry))

		cl := http.Client{
			Transport: httptransport.HttpMetricCollectingRoundTripper{RoundTripper: r},
		}

		ctx := httptransport.ContextWithMetrics(context.Background(), CommonRequestMetricsConfig(config.ServiceProviderType{Name: "test"}, "testOp"))

		req, err := http.NewRequestWithContext(ctx, "GET", "https://test.url/some/path", strings.NewReader(""))
		assert.NoError(t, err)

		_, _ = cl.Do(req)

		mfs, err := registry.Gather()
		assert.NoError(t, err)

		for _, mf := range mfs {
			for _, m := range mf.GetMetric() {
				// check that all the metrics with some nonzero value have the expected labels
				if m.Histogram != nil {
					if m.Histogram.SampleCount == nil || *m.Histogram.SampleCount == 0 {
						continue
					}
				} else if m.Counter != nil {
					if m.Counter.Value == nil || *m.Counter.Value == 0 {
						continue
					}
				} else {
					assert.Fail(t, "only expecting histogram or counter metrics")
				}

				for _, lp := range m.Label {
					assert.Equal(t, expectedLabels[*lp.Name], *lp.Value)
				}
			}
		}
	}

	t.Run("collects successful requests", func(t *testing.T) {
		test(t, func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 42}, nil
		}, map[string]string{"sp": "test", "hostname": "test.url", "method": "GET", "failure": "false", "status": "42", "operation": "testOp"})

	})

	t.Run("collects unsuccessful requests", func(t *testing.T) {
		test(t, func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("intentional request error")
		}, map[string]string{"sp": "test", "hostname": "test.url", "method": "GET", "failure": "true", "operation": "testOp"})
	})
}
