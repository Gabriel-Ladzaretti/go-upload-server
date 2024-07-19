package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
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

// Config holds the configuration settings for the application.
type Config struct {
	dir          string        // Directory where files are saved.
	listenAddr   string        // Port on which the server listens.
	readTimeout  time.Duration // Timeout for reading the request.
	writeTimeout time.Duration // Timeout for writing the response.
	idleTimeout  time.Duration // Timeout for keeping idle connections.
}

func (c Config) String() string {
	return fmt.Sprintf("Config{dir: %s, listenAddr: %s, readTimeout: %v, writeTimeout: %v, idleTimeout: %v}", c.dir, c.listenAddr, c.readTimeout, c.writeTimeout, c.idleTimeout)
}

// newConfig parses command-line flags and returns a Config instance.
func newConfig() Config {
	c := Config{}

	flag.StringVar(&c.dir, "dir", "/tmp", "A path to the directory where files are saved to.")
	flag.StringVar(&c.listenAddr, "listen-addr", ":5000", "The port to listen at.")
	flag.DurationVar(&c.readTimeout, "read-timeout", 15*time.Second, "Timeout for reading the request.")
	flag.DurationVar(&c.writeTimeout, "write-timeout", 15*time.Second, "Timeout for writing the response.")
	flag.DurationVar(&c.idleTimeout, "idle-timeout", 60*time.Second, "Timeout for keeping idle connections.")

	flag.Parse()

	return c
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

// addRoutes configures the routes for the HTTP server.
func addRoutes(mux *http.ServeMux) {
	mux.Handle("/", http.NotFoundHandler())
	mux.Handle("/healthz", healthz())
}

// healthz returns an HTTP handler that checks the health status of the application.
// It responds with 200 OK if the application is healthy, and 503 Service Unavailable otherwise.
func healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&healthy) == 1 {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

// Middleware is a function that wraps http.Handlers
// proving functionality before and after execution
// of the h handler.
type Middleware func(h http.Handler) http.Handler

// NewLoggingMiddleware creates a middleware that logs HTTP requests.
func NewLoggingMiddleware(logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Println(requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			}()

			next.ServeHTTP(w, r)
		})
	}
}

type RequestIDFunc func() string

// generateRequestID generates a unique request ID based on the current time.
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// NewTracingMiddleware creates a middleware that sets and
// propagates a request ID through the request context and response header.
func NewTracingMiddleware(requestIDFunc RequestIDFunc) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")

			if len(requestID) == 0 {
				if requestIDFunc != nil {
					requestID = requestIDFunc()
				} else {
					requestID = generateRequestID()
				}
			}

			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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
