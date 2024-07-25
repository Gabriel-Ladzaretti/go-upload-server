package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"time"
)

// key defines a type for context keys used in the application.
type key int

const (
	requestIDKey key = 0 // requestIDKey is used to store the request ID in the context.
)

var (
	config  Config      // config holds the configuration settings for the application.
	logger  *log.Logger // logger is the default logger used.
	healthy int32       // healthy indicates the health status of the application.
)

// mustInitialize sets up the configuration and performs necessary checks.
// Logs any errors and exits if encountered.
func mustInitialize() {
	logger = log.New(os.Stdout, "http: ", log.LstdFlags)

	config = newConfig()

	fi, err := os.Stat(config.dir)
	if err != nil {
		logger.Fatalf("Error checking configured directory: %v", err)
	}

	if !fi.IsDir() {
		logger.Fatalf("Configured path is not a directory: %s", config.dir)
	}
}

func main() {
	mustInitialize()

	logger.Printf("Initialization completed successfully; Server config: %s", config)

	srv := newServer(logger, config, nil)

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
	dir             string        // dir is the directory where files are saved.
	listenAddr      string        // listenAddr on which the server listens.
	formUploadField string        // formUploadField is the name of the form field used for file uploads.
	uploadEndpoint  string        // uploadEndpoint is the path the to file upload endpoint.
	maxInMemorySize int64         // maxInMemorySize bytes of the file parts are stored in memory, with the remainder stored on disk in temporary files.
	readTimeout     time.Duration // readTimeout is the timeout value for reading the request
	writeTimeout    time.Duration // writeTimeout is the timeout value for writing the response
	idleTimeout     time.Duration // idleTimeout is the timeout for keeping idle connections
}

// String returns a formatted string of the configuration fields.
func (c Config) String() string {
	return fmt.Sprintf(
		"Config{dir: %s, listenAddr: %s, formUploadField: %s, uploadEndpoint: %s, maxInMemorySize: %dB, readTimeout: %v, writeTimeout: %v, idleTimeout: %v}",
		c.dir, c.listenAddr, c.formUploadField, c.uploadEndpoint, c.maxInMemorySize, c.readTimeout, c.writeTimeout, c.idleTimeout,
	)
}

// newConfig parses command-line flags and returns a Config instance.
func newConfig() Config {
	c := Config{}

	flag.StringVar(&c.dir, "dir", "/tmp", "A path to the directory where files are saved to (default: '/tmp').")
	flag.StringVar(&c.listenAddr, "listen-addr", ":3000", "Address for the server to listen on, in the form 'host:port'. (default: ':3000').")
	flag.StringVar(&c.formUploadField, "form-field", "upload", "The name of the form field used for file uploads (default: 'upload').")
	flag.StringVar(&c.uploadEndpoint, "upload-endpoint", "/upload", "The path to the upload API endpoint (default: '/upload').")
	flag.Int64Var(&c.maxInMemorySize, "max-size", 10, "The maximum memory size (in megabytes) for storing part files in memory (default: 10).")
	flag.DurationVar(&c.readTimeout, "read-timeout", 15*time.Second, "Timeout for reading the request (default: '15s').")
	flag.DurationVar(&c.writeTimeout, "write-timeout", 15*time.Second, "Timeout for writing the response (default: '15s').")
	flag.DurationVar(&c.idleTimeout, "idle-timeout", 60*time.Second, "Timeout for keeping idle connections (default: '60s').")

	flag.Parse()

	c.maxInMemorySize <<= 20 // convert to MB

	return c
}

// newServer creates a new HTTP server with middleware.
func newServer(logger *log.Logger, config Config, nextRequestID RequestIDFunc) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, config)

	var handler http.Handler = mux
	handler = NewLoggingMiddleware(logger)(handler)
	handler = NewTracingMiddleware(nextRequestID)(handler)

	return handler
}

// addRoutes configures the routes for the HTTP server.
func addRoutes(mux *http.ServeMux, config Config) {
	mux.Handle("/", http.NotFoundHandler())
	mux.Handle("/healthz", healthz())
	mux.Handle(config.uploadEndpoint, upload(config.dir, config.formUploadField, config.maxInMemorySize))

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

// upload handles file uploads from multipart forms.
func upload(baseDir, formFileFieldName string, maxFileSize int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseMultipartForm(maxFileSize)
		if err != nil {
			logger.Printf("Error parsing multipart form: %v", err)
			http.Error(w, "Could not parse multipart form", http.StatusBadRequest)
			return
		}

		file, handler, err := r.FormFile(formFileFieldName)
		if err != nil {
			logger.Printf("Error retrieving file from form: %v", err)
			http.Error(w, "Could not get file from form", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Create a new file in the uploads directory
		path := filepath.Join(baseDir, handler.Filename)
		dst, err := os.Create(path)
		if err != nil {
			logger.Printf("Error creating file on disk: %v", err)
			http.Error(w, "Could not create file on disk", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		// Copy the uploaded file to the new file
		_, err = io.Copy(dst, file)
		if err != nil {
			log.Printf("Error saving file: %v", err)
			http.Error(w, "Could not save file", http.StatusInternalServerError)
			return
		}

		logger.Printf("File uploaded successfully: %s\n", handler.Filename)
		fmt.Fprintf(w, "File uploaded successfully: %s\n", handler.Filename)
	})
}

// Middleware is a function that wraps [http.Handler]s
// proving functionality before or/and after execution
// of the h handler.
type Middleware func(h http.Handler) http.Handler

// NewLoggingMiddleware creates a middleware that logs HTTP requests.
func NewLoggingMiddleware(logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func(start time.Time) {
				elapsed := time.Since(start)
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Println(requestID, r.Method, r.URL.Path, elapsed, r.RemoteAddr, r.UserAgent())
			}(time.Now())

			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDFunc is a function type for generating unique request IDs,
// used in the tracing middleware [NewTracingMiddleware].
type RequestIDFunc func() string

// defaultRequestIDFunc generates a unique request ID based on the current time.
// It is the default [RequestIDFunc] used if none is provided for the tracing middleware.
func defaultRequestIDFunc() string {
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
					requestID = defaultRequestIDFunc()
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
