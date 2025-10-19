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
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"qwen-go-proxy/internal/infrastructure/logging"
)

// responseWriterWrapper wraps gin.ResponseWriter to capture response body and headers
type responseWriterWrapper struct {
	gin.ResponseWriter
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
func RequestLogging(logger *logging.Logger, debugMode bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		requestID := GetRequestID(c)

		// Enhanced request logging in debug mode
		if debugMode {
			// Log comprehensive request details
			logger.Debug("Debug request started",
				"request_id", requestID,
				"method", c.Request.Method,
				"url", c.Request.URL.String(),
				"host", c.Request.Host,
				"headers", c.Request.Header,
				"content_length", c.Request.ContentLength)

			// Log request body for debugging (be careful with large payloads)
			if c.Request.Body != nil {
				bodyBytes, err := io.ReadAll(c.Request.Body)
				if err != nil {
					logger.Debug("Error reading request body", "request_id", requestID, "error", err)
				} else if len(bodyBytes) > 0 && len(bodyBytes) < 1024*10 { // Only log if under 10KB
					logger.Debug("Request body", "request_id", requestID, "size", len(bodyBytes), "body", string(bodyBytes))
				} else if len(bodyBytes) >= 1024*10 {
					logger.Debug("Request body too large to log", "request_id", requestID, "size", len(bodyBytes))
				}
				// Always restore the request body for further processing
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			} else {
				logger.Debug("Request body empty", "request_id", requestID)
			}
		} else {
			logger.Info("Request started", "request_id", requestID, "method", c.Request.Method, "path", path)
		}

		// Wrap response writer to capture response details in debug mode
		var responseWrapper *responseWriterWrapper
		if debugMode {
			responseWrapper = &responseWriterWrapper{
				ResponseWriter: c.Writer,
				body:           bytes.NewBuffer([]byte{}),
				status:         200, // default status
			}
			c.Writer = responseWrapper
		}

		// Process request
		c.Next()

		// Enhanced response logging
		end := time.Now()
		latency := end.Sub(start)

		if raw != "" {
			path = path + "?" + raw
		}

		statusCode := c.Writer.Status()

		if debugMode {
			// Log comprehensive response details
			headers := make(map[string][]string)
			for key, values := range c.Writer.Header() {
				headers[key] = values
			}

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
	}
}

// RequestID adds a unique request ID to each request for tracing and debugging
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate a unique request ID
		requestID := generateRequestID()

		// Add to response header
		c.Header("X-Request-ID", requestID)

		// Add to gin context for use in handlers
		c.Set("request_id", requestID)

		// Add to request context for propagation
		ctx := context.WithValue(c.Request.Context(), "request_id", requestID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
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

// GetRequestID retrieves the request ID from gin context
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return "unknown"
}

// RateLimit implements rate limiting middleware with improved concurrency and cleanup
func RateLimit(requestsPerSecond int, burst int) gin.HandlerFunc {
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

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
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
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerSecond))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", now.Add(time.Second).Unix()))
			c.Header("Retry-After", "1")

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"message": "Rate limit exceeded",
					"type":    "rate_limit_error",
					"code":    "rate_limit_exceeded",
				},
			})
			c.Abort()
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
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerSecond))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", now.Add(time.Second).Unix()))

		c.Next()
	}
}

// RequestTracker holds request times and mutex for a specific IP
type RequestTracker struct {
	Requests []time.Time
	Mu       *sync.Mutex
}

// CORS returns CORS middleware
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
