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

package oauth

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"time"
)

var (
	// HttpServiceRequestCountMetric is the metric that collects the request counts for OAuth Service.
	HttpServiceRequestCountMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.MetricsNamespace,
			Subsystem: config.MetricsSubsystem,
			Name:      "oauth_service_requests_total",
			Help:      "The request counts to OAuth service categorized by HTTP method status code.",
		},
		[]string{"code", "method"},
	)

	OAuthFlowCompleteTimeMetric = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: config.MetricsNamespace,
		Subsystem: config.MetricsSubsystem,
		Name:      "oauth_flow_complete_time_seconds",
		Help:      "The time needed to complete OAuth flow provider and status code",
	}, []string{"provider", "status"})
)

// HttpServiceInstrumentMetricHandler is a http.Handler that collects statistical information about
// incoming HTTP request and store it in prometheus.Registerer.
func HttpServiceInstrumentMetricHandler(reg prometheus.Registerer, handler http.Handler) http.Handler {
	reg.MustRegister(HttpServiceRequestCountMetric)
	return promhttp.InstrumentHandlerCounter(HttpServiceRequestCountMetric, handler)
}

type StatusHeaderResponseWriter struct {
	wrapped    http.ResponseWriter
	StatusCode int
}

func NewStatusHeaderResponseWriter(w http.ResponseWriter) *StatusHeaderResponseWriter {
	return &StatusHeaderResponseWriter{
		wrapped:    w,
		StatusCode: 200,
	}
}

func (w *StatusHeaderResponseWriter) Wrapped() http.ResponseWriter {
	return w.wrapped
}

func (w *StatusHeaderResponseWriter) Header() http.Header {
	return w.wrapped.Header()
}

func (w *StatusHeaderResponseWriter) Write(buf []byte) (int, error) {
	return w.wrapped.Write(buf)
}

func (w *StatusHeaderResponseWriter) WriteHeader(statusCode int) {
	w.wrapped.WriteHeader(statusCode)
	w.StatusCode = statusCode
}

var _ http.ResponseWriter = &StatusHeaderResponseWriter{}

type CompletedFlowMetricHandler struct {
	spConfiguration config.ServiceProviderConfiguration
	stateStorage    *StateStorage
	handler         http.Handler
}

func NewCompletedFlowMetricHandler(reg prometheus.Registerer, spConfiguration config.ServiceProviderConfiguration, stateStorage *StateStorage, handler http.Handler) *CompletedFlowMetricHandler {
	reg.MustRegister(OAuthFlowCompleteTimeMetric)
	return &CompletedFlowMetricHandler{spConfiguration: spConfiguration, stateStorage: stateStorage, handler: handler}
}

func (c *CompletedFlowMetricHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log := log.FromContext(req.Context())
	log.Info("CompletedFlowMetricHandler ServeHTTP")
	statusWriter := NewStatusHeaderResponseWriter(w)
	c.handler.ServeHTTP(statusWriter, req)
	veiledAt, err := c.stateStorage.StateVeiledAt(req.Context(), req)
	if err != nil {
		log.Error(err, "Unknown state")
	}
	OAuthFlowCompleteTimeMetric.WithLabelValues(string(c.spConfiguration.ServiceProviderType), strconv.Itoa(statusWriter.StatusCode)).Observe(time.Since(veiledAt).Seconds())

}
