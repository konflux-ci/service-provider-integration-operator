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
			Help:      "Total number of spi oauth service requests by HTTP code.",
		},
		[]string{"code", "method"},
	)
	reg.MustRegister(reqCounter)
	return promhttp.InstrumentHandlerCounter(reqCounter, handler)
}
