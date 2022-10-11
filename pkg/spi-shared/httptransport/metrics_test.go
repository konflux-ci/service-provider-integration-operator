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

package httptransport

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/util"
	"github.com/stretchr/testify/assert"
)

func TestMetricsCollectingRoundTripper(t *testing.T) {
	req, err := http.NewRequest("GET", "https://over.the.rainbow", strings.NewReader(""))
	assert.NoError(t, err)

	registry := prometheus.NewPedanticRegistry()

	roundtripperCalled := false
	tr := HttpMetricCollectingRoundTripper{RoundTripper: util.FakeRoundTrip(func(r *http.Request) (*http.Response, error) {
		roundtripperCalled = true
		return &http.Response{
			Status:     "status",
			StatusCode: 42,
			Request:    r,
		}, nil
	})}

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "test",
		Subsystem: "test",
		Name:      "gauge",
	})

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "test",
		Subsystem: "test",
		Name:      "counter",
	})

	histogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "test",
		Subsystem: "test",
		Name:      "histogram",
	})

	summary := prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace: "test",
		Subsystem: "test",
		Name:      "summary",
	})

	registry.MustRegister(counter, gauge, histogram, summary)

	ctx := ContextWithMetrics(context.Background(), &HttpMetricCollectionConfig{
		GaugePicker: HttpGaugeMetricPickerFunc(func(request *http.Request, response *http.Response, err error) []prometheus.Gauge {
			return []prometheus.Gauge{gauge}
		}),
		CounterPicker: HttpCounterMetricPickerFunc(func(request *http.Request, response *http.Response, err error) []prometheus.Counter {
			return []prometheus.Counter{counter}
		}),
		HistogramOrSummaryPicker: HttpHistogramOrSummaryMetricPickerFunc(func(request *http.Request, response *http.Response, err error) []prometheus.Observer {
			return []prometheus.Observer{histogram, summary}
		}),
	})

	req = req.WithContext(ctx)

	_, _ = tr.RoundTrip(req)

	var counterPresent bool
	var gaugePresent bool
	var histoPresent bool
	var summaryPresent bool

	gathered, err := registry.Gather()
	assert.NoError(t, err)

	for _, mf := range gathered {
		for _, m := range mf.GetMetric() {
			if m.Histogram != nil {
				assert.Equal(t, *m.Histogram.SampleCount, uint64(1))
				histoPresent = true
			}
			if m.Summary != nil {
				assert.Equal(t, *m.Summary.SampleCount, uint64(1))
				summaryPresent = true
			}
			if m.Counter != nil {
				assert.Equal(t, *m.Counter.Value, 1.0)
				counterPresent = true
			}
			if m.Gauge != nil {
				assert.Greater(t, *m.Gauge.Value, 0.0)
				gaugePresent = true
			}
		}
	}

	assert.True(t, roundtripperCalled)
	assert.True(t, counterPresent)
	assert.True(t, gaugePresent)
	assert.True(t, histoPresent)
	assert.True(t, summaryPresent)
}
