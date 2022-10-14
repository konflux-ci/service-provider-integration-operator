package metrics

import (
	"context"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"net/http"
	"os"
	"os/signal"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RegistererGatherer combines both parts of the API of a Prometheus
// registry, both the Registerer and the Gatherer interfaces.
type RegistererGatherer interface {
	prometheus.Registerer
	prometheus.Gatherer
}

// Registry is a prometheus registry for storing metrics within the
// controller-runtime.
var Registry RegistererGatherer = prometheus.NewRegistry()

func init() {
	log := log.FromContext(context.TODO())
	log.Info("setup MustRegister")
	Registry.MustRegister(
		// expose process metrics like CPU, Memory, file descriptor usage etc.
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		// expose Go runtime metrics like GC stats, memory stats etc.
		collectors.NewGoCollector(),
	)
	log.Info("setup MustRegister2")
}

func ServeMetrics(ctx context.Context, address string) {
	setupLog := ctrl.Log.WithName("metrics")
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.InstrumentMetricHandler(Registry, promhttp.Handler()))
	server := &http.Server{Addr: address,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout:      time.Second * 15,
		ReadTimeout:       time.Second * 15,
		ReadHeaderTimeout: time.Second * 15,
		IdleTimeout:       time.Second * 60,
		Handler:           mux,
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := server.ListenAndServe(); err != nil {
			setupLog.Error(err, "failed to start the metrics HTTP server")
		}
	}()
	setupLog.Info("Metrics server is up and running", "Addr", address)
	// Setting up signal capturing
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Waiting for SIGINT (kill -2)
	<-stop
	setupLog.Info("Server got interrupt signal, going to gracefully shutdown the server", "signal", stop)
	// Create a deadline to wait for.
	context, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	if err := server.Shutdown(context); err != nil {
		setupLog.Error(err, "Metrics server shutdown failed")
		return
	}
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	setupLog.Info("Metrics server exited properly")
}
