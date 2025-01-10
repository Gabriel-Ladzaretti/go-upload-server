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

const (
	applicationName = "usrv" // applicationName is the cmd/binary name for logging purpose
)

var (
	config  ServerConfig
	logger  *log.Logger
	healthy atomic.Bool
)

func mustInitialize() {
	logger = log.New(os.Stdout, fmt.Sprintf("%s :", applicationName), log.LstdFlags)

	parseConfig(&config)

	fi, err := os.Stat(config.saveDir)
	if err != nil {
		logger.Fatalf("Error checking configured directory: %v", err)
	}

	if !fi.IsDir() {
		logger.Fatalf("Configured path is not a directory: %s", config.saveDir)
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

type ServerConfig struct {
	saveDir      string        // saveDir is where files are saved.
	listenAddr   string        // listenAddr on which the server listens.
	formKey      string        // formKey is the name of the form field used for file uploads.
	endpoint     string        // uploadEndpoint is the path the to file upload endpoint.
	maxInMemory  int64         // maxInMemory is the maximum size to store in memory, the reminder is stored on disk.
	readTimeout  time.Duration // readTimeout is the timeout value for reading the request
	writeTimeout time.Duration // writeTimeout is the timeout value for writing the response
	idleTimeout  time.Duration // idleTimeout is the timeout for keeping idle connections
}

func (c ServerConfig) String() string {
	return fmt.Sprintf(
		"Config{dir: %s, listenAddr: %s, formUploadField: %s, uploadEndpoint: %s, maxInMemorySize: %dB, readTimeout: %v, writeTimeout: %v, idleTimeout: %v}",
		c.saveDir, c.listenAddr, c.formKey, c.endpoint, c.maxInMemory, c.readTimeout, c.writeTimeout, c.idleTimeout,
	)
}

func parseConfig(c *ServerConfig) {
	flag.StringVar(&c.saveDir, "dir", "/tmp", "A path to the directory where files are saved to (default: '/tmp').")
	flag.StringVar(&c.listenAddr, "listen-addr", ":3000", "Address for the server to listen on, in the form 'host:port'. (default: ':3000').")
	flag.StringVar(&c.formKey, "form-field", "upload", "The name of the form field used for file uploads (default: 'upload').")
	flag.StringVar(&c.endpoint, "upload-endpoint", "/upload", "The path to the upload API endpoint (default: '/upload').")
	flag.DurationVar(&c.readTimeout, "read-timeout", 15*time.Second, "Timeout for reading the request (default: '15s').")
	flag.DurationVar(&c.writeTimeout, "write-timeout", 15*time.Second, "Timeout for writing the response (default: '15s').")
	flag.DurationVar(&c.idleTimeout, "idle-timeout", 60*time.Second, "Timeout for keeping idle connections (default: '60s').")
	flag.Int64Var(&c.maxInMemory, "max-size", 10, "The maximum memory size (Mib) for storing part files in memory (default: 10).")

	flag.Parse()

	c.maxInMemory <<= 20 // convert Mib to bytes
}

func newServer(logger *log.Logger, config ServerConfig, nextRequestID RequestIDFunc) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.NotFoundHandler())
	mux.Handle("/healthz", healthz())
	mux.Handle(config.endpoint, upload(config.saveDir, config.formKey, config.maxInMemory))

	var handler http.Handler = mux
	handler = NewLoggingMiddleware(logger)(handler)
	handler = NewTracingMiddleware(nextRequestID)(handler)

	return handler
}

func healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if healthy.Load() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

// upload handles multiform file uploads.
func upload(baseDir string, formKey string, maxMemorySize int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseMultipartForm(maxMemorySize)
		if err != nil {
			logger.Printf("Error parsing multipart form: %v", err)
			http.Error(w, "Could not parse multipart form", http.StatusBadRequest)
			return
		}

		file, handler, err := r.FormFile(formKey)
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

type Middleware func(h http.Handler) http.Handler

// NewLoggingMiddleware creates a middleware that logs HTTP requests.
func NewLoggingMiddleware(logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer logRequest(r, time.Now())
			next.ServeHTTP(w, r)
		})
	}
}

type RequestID int

const currentRequest RequestID = 0

func logRequest(r *http.Request, start time.Time) {
	elapsed := time.Since(start)
	requestID, ok := r.Context().Value(currentRequest).(string)
	if !ok {
		requestID = "unknown"
	}
	logger.Println(requestID, r.Method, r.URL.Path, elapsed, r.RemoteAddr, r.UserAgent())
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

			ctx := context.WithValue(r.Context(), currentRequest, requestID)
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
