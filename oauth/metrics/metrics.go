package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
)

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
