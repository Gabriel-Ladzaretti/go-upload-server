package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

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
