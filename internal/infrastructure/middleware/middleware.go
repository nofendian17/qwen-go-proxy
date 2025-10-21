package middleware

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"qwen-go-proxy/internal/infrastructure/logging"
)

// Define a custom type for context keys to avoid collisions
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
)

// responseWriterWrapper wraps http.ResponseWriter to capture response body and headers
type responseWriterWrapper struct {
	http.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (rw *responseWriterWrapper) Write(data []byte) (int, error) {
	rw.body.Write(data)
	return rw.ResponseWriter.Write(data)
}

func (rw *responseWriterWrapper) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, errors.New("underlying response writer does not implement http.Hijacker")
}

// RequestLogging returns request logging middleware
func RequestLogging(logger *logging.Logger, debugMode bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			path := r.URL.Path
			raw := r.URL.RawQuery
			requestID := GetRequestID(r.Context())

			// Enhanced request logging in debug mode
			if debugMode {
				// Log comprehensive request details
				logger.Debug("Debug request started",
					"request_id", requestID,
					"method", r.Method,
					"url", r.URL.String(),
					"host", r.Host,
					"headers", r.Header,
					"content_length", r.ContentLength)

				// Log request body for debugging (be careful with large payloads)
				if r.Body != nil {
					bodyBytes, err := io.ReadAll(r.Body)
					if err != nil {
						logger.Debug("Error reading request body", "request_id", requestID, "error", err)
					} else if len(bodyBytes) > 0 && len(bodyBytes) < 1024*10 { // Only log if under 10KB
						logger.Debug("Request body", "request_id", requestID, "size", len(bodyBytes), "body", string(bodyBytes))
					} else if len(bodyBytes) >= 1024*10 {
						logger.Debug("Request body too large to log", "request_id", requestID, "size", len(bodyBytes))
					}
					// Always restore the request body for further processing
					r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				} else {
					logger.Debug("Request body empty", "request_id", requestID)
				}
			} else {
				logger.Info("Request started", "request_id", requestID, "method", r.Method, "path", path)
			}

			// Wrap response writer to capture response details in debug mode
			var responseWrapper *responseWriterWrapper
			if debugMode {
				responseWrapper = &responseWriterWrapper{
					ResponseWriter: w,
					body:           bytes.NewBuffer([]byte{}),
					status:         200, // default status
				}
				w = responseWrapper
			}

			// Create a new request with the wrapped writer
			next.ServeHTTP(w, r)

			// Enhanced response logging
			end := time.Now()
			latency := end.Sub(start)

			if raw != "" {
				path = path + "?" + raw
			}

			var statusCode int
			if responseWrapper != nil {
				statusCode = responseWrapper.status
			}
			if statusCode == 0 { // fallback if wrapper wasn't used
				if wrappedWriter, ok := w.(*responseWriterWrapper); ok {
					statusCode = wrappedWriter.status
				}
			}
			if statusCode == 0 { // final fallback
				statusCode = 200
			}

			if debugMode {
				// Log comprehensive response details
				headers := w.Header()

				logger.Debug("Debug response completed",
					"request_id", requestID,
					"status", statusCode,
					"latency", latency,
					"headers", headers)

				// Log response body (if captured)
				if responseWrapper != nil && responseWrapper.body.Len() > 0 {
					responseBody := responseWrapper.body.String()
					if len(responseBody) > 1024*5 { // Limit body logging to 5KB
						logger.Debug("Response body truncated", "request_id", requestID, "size", len(responseBody), "body", responseBody[:1024*5]+"...")
					} else {
						logger.Debug("Response body", "request_id", requestID, "size", len(responseBody), "body", responseBody)
					}
				}
			} else {
				logger.Info("Request completed", "request_id", requestID, "status", statusCode, "path", path, "latency", latency)
			}
		})
	}
}

// RequestID adds a unique request ID to each request for tracing and debugging
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate a unique request ID
			requestID := generateRequestID()

			// Add to response header
			w.Header().Set("X-Request-ID", requestID)

			// Add to request context for propagation
			ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// generateRequestID creates a unique request identifier
func generateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto rand fails
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	if ctxRequestID := ctx.Value(RequestIDKey); ctxRequestID != nil {
		if id, ok := ctxRequestID.(string); ok {
			return id
		}
	}
	return "unknown"
}

// RateLimit implements rate limiting middleware with improved concurrency and cleanup
func RateLimit(requestsPerSecond int, burst int) func(http.Handler) http.Handler {
	// Use sync.Map for concurrent access without global mutex bottleneck
	var requestCounts sync.Map

	// Start cleanup goroutine to prevent memory leaks
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()
			cutoff := now.Add(-10 * time.Minute) // Remove entries older than 10 minutes

			requestCounts.Range(func(key, value interface{}) bool {
				tracker := value.(*RequestTracker)
				tracker.Mu.Lock()
				// Remove tracker if no recent requests
				if len(tracker.Requests) == 0 ||
					(len(tracker.Requests) > 0 && tracker.Requests[len(tracker.Requests)-1].Before(cutoff)) {
					tracker.Mu.Unlock()
					requestCounts.Delete(key)
				} else {
					tracker.Mu.Unlock()
				}
				return true
			})
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := getClientIP(r)
			now := time.Now()

			// Get or create tracker for this IP
			trackerValue, _ := requestCounts.LoadOrStore(clientIP, &RequestTracker{
				Requests: make([]time.Time, 0, burst),
				Mu:       &sync.Mutex{},
			})
			tracker := trackerValue.(*RequestTracker)

			tracker.Mu.Lock()

			// Remove requests older than 1 second (sliding window)
			cutoff := now.Add(-time.Second)
			validRequests := make([]time.Time, 0, len(tracker.Requests))
			for _, reqTime := range tracker.Requests {
				if reqTime.After(cutoff) {
					validRequests = append(validRequests, reqTime)
				}
			}
			tracker.Requests = validRequests

			// Check if we've exceeded the rate limit
			if len(tracker.Requests) >= requestsPerSecond {
				tracker.Mu.Unlock()
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerSecond))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", now.Add(time.Second).Unix()))
				w.Header().Set("Retry-After", "1")

				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error": {"message": "Rate limit exceeded", "type": "rate_limit_error", "code": "rate_limit_exceeded"}}`))
				return
			}

			// Add current request
			tracker.Requests = append(tracker.Requests, now)
			remaining := requestsPerSecond - len(tracker.Requests)
			if remaining < 0 {
				remaining = 0
			}

			tracker.Mu.Unlock()

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerSecond))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", now.Add(time.Second).Unix()))

			next.ServeHTTP(w, r)
		})
	}
}

// RequestTracker holds request times and mutex for a specific IP
type RequestTracker struct {
	Requests []time.Time
	Mu       *sync.Mutex
}

// CORS returns CORS middleware
func CORS() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the real client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, the first one is the original client
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
