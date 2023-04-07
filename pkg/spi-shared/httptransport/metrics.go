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
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// HttpGaugeMetricPicker is an interface used by the HttpMetricCollectingRoundTripper to pick the gauge metrics
// to collect based on the request and response. Because it is used during an HTTP round trip, the implementations
// of this interface must be thread-safe.
type HttpGaugeMetricPicker interface {
	Pick(r *http.Request, resp *http.Response, err error) []prometheus.Gauge
}

// HttpGaugeMetricPickerFunc is a functional implementation of HttpGaugeMetricPicker
type HttpGaugeMetricPickerFunc func(*http.Request, *http.Response, error) []prometheus.Gauge

func (h HttpGaugeMetricPickerFunc) Pick(r *http.Request, resp *http.Response, err error) []prometheus.Gauge {
	return h(r, resp, err)
}

var _ HttpGaugeMetricPicker = (HttpGaugeMetricPickerFunc)(nil)

// HttpCounterMetricPicker is an interface used by the HttpMetricCollectingRoundTripper to pick the counter metrics
// to collect based on the request and response. Because it is used during an HTTP round trip, the implementations
// of this interface must be thread-safe.
type HttpCounterMetricPicker interface {
	Pick(r *http.Request, resp *http.Response, err error) []prometheus.Counter
}

// HttpCounterMetricPickerFunc is a functional implementation of HttpCounterMetricPicker.
type HttpCounterMetricPickerFunc func(*http.Request, *http.Response, error) []prometheus.Counter

func (h HttpCounterMetricPickerFunc) Pick(r *http.Request, resp *http.Response, err error) []prometheus.Counter {
	return h(r, resp, err)
}

var _ HttpGaugeMetricPicker = (HttpGaugeMetricPickerFunc)(nil)

// HttpHistogramOrSummaryMetricPicker is an interface used by the HttpMetricCollectingRoundTripper to pick the histogram
// or summary metrics to collect based on the request and response. Because it is used during an HTTP round trip,
// the implementations of this interface must be thread-safe.
type HttpHistogramOrSummaryMetricPicker interface {
	Pick(r *http.Request, resp *http.Response, err error) []prometheus.Observer
}

// HttpHistogramOrSummaryMetricPickerFunc is a functional implementation of HttpHistogramOrSummaryMetricPicker.
type HttpHistogramOrSummaryMetricPickerFunc func(*http.Request, *http.Response, error) []prometheus.Observer

func (h HttpHistogramOrSummaryMetricPickerFunc) Pick(r *http.Request, resp *http.Response, err error) []prometheus.Observer {
	return h(r, resp, err)
}

var _ HttpGaugeMetricPicker = (HttpGaugeMetricPickerFunc)(nil)

// HttpMetricCollectingRoundTripper is a wrapper around http.RoundTripper interface that, given an HttpMetricCollectionConfig,
// collects the metrics for each HTTP request processed. The metric collection configuration is passed to the roundtripper
// by injecting it into a context, so it can differ per request.
type HttpMetricCollectingRoundTripper struct {
	http.RoundTripper
}

var _ http.RoundTripper = (*HttpMetricCollectingRoundTripper)(nil)

type metricsCollectorContextKeyType struct{}

var metricsCollectorContextKey = metricsCollectorContextKeyType{}

// ContextWithMetrics returns a new context based on the supplied one that contains the provided metrics collection
// configurations. If this context is used to perform an HTTP request with the HttpMetricCollectingRoundTripper, the
// configured metrics will be collected.
func ContextWithMetrics(ctx context.Context, cfg *HttpMetricCollectionConfig) context.Context {
	return context.WithValue(ctx, metricsCollectorContextKey, cfg)
}

// HttpMetricCollectionConfig specifies what metric should be collected.
type HttpMetricCollectionConfig struct {
	GaugePicker              HttpGaugeMetricPicker
	CounterPicker            HttpCounterMetricPicker
	HistogramOrSummaryPicker HttpHistogramOrSummaryMetricPicker
}

func (h HttpMetricCollectingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	stored := request.Context().Value(metricsCollectorContextKey)
	var cfg *HttpMetricCollectionConfig

	if stored != nil {
		cfg = stored.(*HttpMetricCollectionConfig)
	}

	if cfg == nil {
		//nolint:wrapcheck // we're returning the error from the underlying http round trip. this IMHO should not be wrapped.
		return h.RoundTripper.RoundTrip(request)
	}

	start := time.Now()

	response, err := h.RoundTripper.RoundTrip(request)

	dur := time.Since(start).Seconds()

	if cfg.GaugePicker != nil {
		gs := cfg.GaugePicker.Pick(request, response, err)
		for _, g := range gs {
			g.Set(dur)
		}
	}

	if cfg.CounterPicker != nil {
		cs := cfg.CounterPicker.Pick(request, response, err)
		for _, c := range cs {
			c.Inc()
		}
	}

	if cfg.HistogramOrSummaryPicker != nil {
		hs := cfg.HistogramOrSummaryPicker.Pick(request, response, err)
		for _, h := range hs {
			h.Observe(dur)
		}
	}

	//nolint:wrapcheck // we're returning the error from the underlying http round trip. this IMHO should not be wrapped.
	return response, err
}
