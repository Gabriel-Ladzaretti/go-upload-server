package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

// key defines a type for context keys used in the application.
type key int

const (
	requestIDKey key = 0 // requestIDKey is used to store the request ID in the context.
)

var (
	config  Config // config holds the configuration settings for the application.
	healthy int32  // healthy indicates the health status of the application.
)

func main() {
	config = newConfig()
	logger := log.New(os.Stdout, "http: ", log.LstdFlags)

	logger.Printf("Server config: %s", config)

	srv := newServer(logger, nil)

	httpServer := &http.Server{
		Addr:         config.listenAddr,
		Handler:      srv,
		ErrorLog:     logger,
		ReadTimeout:  config.readTimeout,
		WriteTimeout: config.writeTimeout,
		IdleTimeout:  config.idleTimeout,
	}

	ctx := context.Background()
	run(ctx, logger, httpServer)
}

// newServer creates a new HTTP server with middleware.
func newServer(logger *log.Logger, nextRequestID RequestIDFunc) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux)

	var handler http.Handler = mux
	handler = NewLoggingMiddleware(logger)(handler)
	handler = NewTracingMiddleware(nextRequestID)(handler)

	return handler
}

// run starts the HTTP server and handles graceful shutdown.
func run(ctx context.Context, logger *log.Logger, httpServer *http.Server) {
	go func() {
		logger.Printf("listening on %s\n", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "error listening and serving: %s\n", err)
		}
	}()

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	<-ctx.Done()

	logger.Println("Shutting down gracefully, press Ctrl+C again to force")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "error shutting down http server: %s\n", err)
	}

	logger.Println("Server shut down successfully")
}
