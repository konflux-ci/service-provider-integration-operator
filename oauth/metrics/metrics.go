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

package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
)

// OAuthServiceInstrumentMetricHandler is a http.Handler that collects statistical information about
// incoming HTTP request and store it in prometheus.Registerer.
func OAuthServiceInstrumentMetricHandler(reg prometheus.Registerer, handler http.Handler) http.Handler {
	reqCounter := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.MetricsNamespace,
			Subsystem: config.MetricsSubsystem,
			Name:      "oauth_service_requests_total",
			Help:      "The request counts to OAuth service categorized by HTTP method status code.",
		},
		[]string{"code", "method"},
	)

	reg.MustRegister(reqCounter)
	return promhttp.InstrumentHandlerCounter(reqCounter, handler)
}
