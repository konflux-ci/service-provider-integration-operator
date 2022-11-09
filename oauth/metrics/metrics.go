package metrics

import (
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: config.MetricsNamespace,
		Subsystem: config.MetricsSubsystem,
		Name:      "oauth_service_response_time_seconds",
		Help:      "The response time of OAuth service requests categorized by HTTP method and status code",
	}, []string{"code", "method"})

	reg.MustRegister(reqCounter, duration)
	return promhttp.InstrumentHandlerDuration(duration, promhttp.InstrumentHandlerCounter(reqCounter, handler))
}
