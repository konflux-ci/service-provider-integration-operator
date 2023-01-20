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
	"fmt"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"
)

var (
	// RequestCountMetric is the metric that collects the request counts for all service providers.
	// We allow for the unbounded "hostname" label with the assumption that the real number of service providers will be
	// limited to only a couple in practice.
	//
	// The `operation` label signifies the operation being performed in the controller for which we need to contact the
	// service provider.
	//
	// Note that while this metric may seem similar to the automatic _count of ResponseTimeMetric histogram, it is different
	// because it counts the request attempts, which should also include requests for which it was not possible to obtain
	// the response (which have the "failure" label set to true).
	//
	// Preferably, use the CommonRequestMetricsConfig function to use this metric and register it using the RegisterCommonMetrics
	// function.
	RequestCountMetric = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: config.MetricsNamespace,
		Subsystem: config.MetricsSubsystem,
		Name:      "service_provider_request_count_total",
		Help:      "The request counts to service providers categorized by service provider type, hostname and HTTP method",
	}, []string{"sp", "hostname", "method", "failure", "operation"})

	// ResponseTimeMetric is the metric that collects the request response times for all service providers.
	// We allow for the unbounded "hostname" label with the assumption that the real number of service providers will be
	// limited to only a couple in practice.
	//
	// The `operation` label signifies the operation being performed in the controller for which we need to contact the
	// service provider.
	//
	// Preferably, use the CommonRequestMetricsConfig function to use this metric and register it using the RegisterCommonMetrics
	// function.
	ResponseTimeMetric = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: config.MetricsNamespace,
		Subsystem: config.MetricsSubsystem,
		Name:      "service_provider_response_time_seconds",
		Help:      "The response time of service provider requests categorized by service provider hostname, HTTP method and status code",
	}, []string{"sp", "hostname", "method", "status", "operation"})
)

// RegisterCommonMetrics registers the RequestCountMetric and ResponseTimeMetric with the provided registerer. This must be
// called exactly once.
func RegisterCommonMetrics(registerer prometheus.Registerer) error {
	if err := registerer.Register(RequestCountMetric); err != nil {
		return fmt.Errorf("failed to register service provider request count metric: %w", err)
	}

	if err := registerer.Register(ResponseTimeMetric); err != nil {
		return fmt.Errorf("failed to register service provider response time metric: %w", err)
	}

	return nil
}

// CommonRequestMetricsConfig returns the metrics collection configuration for collecting the RequestCountMetric and
// ResponseTimeMetric for the provided service provider type with given operation.
//
// The returned configuration can be used with httptransport.ContextWithMetrics to configure what metrics should be
// collected in the http requests.
func CommonRequestMetricsConfig(spType config.ServiceProviderType, operation string) *httptransport.HttpMetricCollectionConfig {
	return &httptransport.HttpMetricCollectionConfig{
		CounterPicker:            requestCountPicker(spType.Name, operation),
		HistogramOrSummaryPicker: responseTimePicker(spType.Name, operation),
	}
}

func requestCountPicker(spType config.ServiceProviderName, operation string) httptransport.HttpCounterMetricPickerFunc {
	return func(request *http.Request, response *http.Response, err error) []prometheus.Counter {
		failed := "false"
		if err != nil || response == nil {
			failed = "true"
		}
		return []prometheus.Counter{RequestCountMetric.WithLabelValues(string(spType), request.Host, request.Method, failed, operation)}
	}
}

func responseTimePicker(spType config.ServiceProviderName, operation string) httptransport.HttpHistogramOrSummaryMetricPickerFunc {
	return func(request *http.Request, resp *http.Response, err error) []prometheus.Observer {
		if resp == nil {
			return nil
		}
		return []prometheus.Observer{ResponseTimeMetric.WithLabelValues(string(spType), request.Host, request.Method, strconv.Itoa(resp.StatusCode), operation)}
	}
}
