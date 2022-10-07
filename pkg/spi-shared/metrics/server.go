package metrics

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"os"
	"os/signal"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//var metricsRegistry *prometheus.Registry

func ServeMetrics(ctx context.Context, address string) {
	setupLog := ctrl.Log.WithName("metrics")
	//metricsRegistry = prometheus.NewRegistry()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{})))
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
			setupLog.Error(err, "failed to start the HTTP server")
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	if err := server.Shutdown(ctx); err != nil {
		setupLog.Error(err, "OAuth server shutdown failed")
		os.Exit(1)
	}
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	setupLog.Info("Metrics server exited properly")
	os.Exit(0)
}
