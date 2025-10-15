package middleware

import (
	"bufio"
	"bytes"
	"errors"
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

		// Enhanced request logging in debug mode
		if debugMode {
			// Log comprehensive request details
			logger.Debug("Debug request started",
				"method", c.Request.Method,
				"url", c.Request.URL.String(),
				"host", c.Request.Host,
				"headers", c.Request.Header,
				"content_length", c.Request.ContentLength)

			// Log request body for debugging (be careful with large payloads)
			if c.Request.Body != nil {
				bodyBytes, err := io.ReadAll(c.Request.Body)
				if err != nil {
					logger.Debug("Error reading request body", "error", err)
				} else if len(bodyBytes) > 0 && len(bodyBytes) < 1024*10 { // Only log if under 10KB
					logger.Debug("Request body", "size", len(bodyBytes), "body", string(bodyBytes))
				} else if len(bodyBytes) >= 1024*10 {
					logger.Debug("Request body too large to log", "size", len(bodyBytes))
				}
				// Always restore the request body for further processing
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			} else {
				logger.Debug("Request body empty")
			}
		} else {
			logger.Info("Request started", "method", c.Request.Method, "path", path)
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
				"status", statusCode,
				"latency", latency,
				"headers", headers)

			// Log response body (if captured)
			if responseWrapper != nil && responseWrapper.body.Len() > 0 {
				responseBody := responseWrapper.body.String()
				if len(responseBody) > 1024*5 { // Limit body logging to 5KB
					logger.Debug("Response body truncated", "size", len(responseBody), "body", responseBody[:1024*5]+"...")
				} else {
					logger.Debug("Response body", "size", len(responseBody), "body", responseBody)
				}
			}
		} else {
			logger.Info("Request completed", "status", statusCode, "path", path, "latency", latency)
		}
	}
}

// RateLimit implements rate limiting middleware
func RateLimit(requestsPerSecond int, burst int) gin.HandlerFunc {
	// Use a map to track request counts per IP address
	requestCounts := make(map[string]*RequestTracker)
	mu := &sync.Mutex{}

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		mu.Lock()
		tracker, exists := requestCounts[clientIP]
		if !exists {
			tracker = &RequestTracker{
				Requests: make([]time.Time, 0, burst),
				Mu:       &sync.Mutex{},
			}
			requestCounts[clientIP] = tracker
		}
		mu.Unlock()

		now := time.Now()
		tracker.Mu.Lock()

		// Remove requests older than 1 second
		cutoff := now.Add(-time.Second)
		newRequests := make([]time.Time, 0, len(tracker.Requests))
		for _, reqTime := range tracker.Requests {
			if reqTime.After(cutoff) {
				newRequests = append(newRequests, reqTime)
			}
		}
		tracker.Requests = newRequests

		// Check if we've exceeded the rate limit
		if len(tracker.Requests) >= requestsPerSecond {
			tracker.Mu.Unlock()
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
		tracker.Mu.Unlock()

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
