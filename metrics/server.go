package metrics

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"time"

	netpprof "net/http/pprof"

	"github.com/celo-org/celo-blockchain/log"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

// Start will start the http metrics server
func StartWithConfig(ctx context.Context, cfg *HttpServerConfig) error {

	handler := defineRoutes(cfg)

	server := &http.Server{
		Addr:         cfg.ListenAddress(),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		// TODO set ErrorLog: ,
	}

	go func() {
		<-ctx.Done()
		if err := server.Close(); err != nil {
			panic("Failed to Stop the service")
		}
	}()

	log.Info("Starting httpServer", "listen_address", cfg.ListenAddress())

	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func defineRoutes(cfg *HttpServerConfig) http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/health", healthCheckHandler)
	r.Handle("/metrics", promhttp.Handler())

	if viper.GetBool("profiling") {
		log.Info("Setting up profiling")

		runtime.SetBlockProfileRate(1000)

		r.HandleFunc("/debug/pprof/", netpprof.Index)
		for _, handler := range []string{
			"allocs", "block", "cmdline", "goroutine", "heap",
			"mutex", "profile", "threadcreate", "trace"} {
			r.HandleFunc(fmt.Sprintf("/debug/pprof/%s", handler), netpprof.Index)
		}
	}

	mainHandler := requestLogHandler(r)
	mainHandler = http.TimeoutHandler(mainHandler, cfg.RequestTimeout, "Request Timed out")

	return mainHandler
}

func Start(ctx context.Context) error {
	return StartWithConfig(ctx, &HttpServerConfig{
		Port:           viper.GetUint("monitoring.port"),
		Interface:      viper.GetString("monitoring.address"),
		RequestTimeout: viper.GetDuration("monitoring.requestTimeoutSeconds") * time.Second,
	})
}
