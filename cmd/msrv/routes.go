package main

import (
	"net/http"
	"sync/atomic"
)

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
