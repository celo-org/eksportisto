package server

import (
	"context"
	"net/http"
	"time"

	"github.com/celo-org/celo-blockchain/log"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Start will start the http metrics server
func Start(ctx context.Context, cfg *HttpServerConfig) error {

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

	mainHandler := requestLogHandler(r)
	mainHandler = http.TimeoutHandler(mainHandler, cfg.RequestTimeout, "Request Timed out")

	return mainHandler
}
