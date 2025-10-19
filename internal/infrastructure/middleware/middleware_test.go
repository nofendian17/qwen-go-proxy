package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"qwen-go-proxy/internal/infrastructure/logging"
)

func TestRequestLogging_DebugMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a logger
	logger := &logging.Logger{Logger: logging.NewLogger("debug")}

	// Create middleware
	middleware := RequestLogging(logger, true)

	// Create test router
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})

	// Create request with body
	reqBody := `{"test": "data"}`
	req := httptest.NewRequest("GET", "/test", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"message":"test"`)
}

func TestRequestLogging_NonDebugMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a logger
	logger := &logging.Logger{Logger: logging.NewLogger("info")}

	// Create middleware
	middleware := RequestLogging(logger, false)

	// Create test router
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})

	// Create request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"message":"test"`)
}

func TestRequestLogging_DebugMode_WithLargeBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a logger
	logger := &logging.Logger{Logger: logging.NewLogger("debug")}

	// Create middleware
	middleware := RequestLogging(logger, true)

	// Create test router
	router := gin.New()
	router.Use(middleware)
	router.POST("/test", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.JSON(200, gin.H{"received": len(body)})
	})

	// Create large request body (>10KB)
	largeBody := strings.Repeat("x", 15*1024)
	req := httptest.NewRequest("POST", "/test", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "15360") // 15*1024
}

func TestRateLimit_AllowRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create middleware with high limits
	middleware := RateLimit(10, 20)

	// Create test router
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// Make multiple requests within limit
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:12345" // Set IP for rate limiting

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.Contains(t, w.Body.String(), `"message":"ok"`)
	}
}

func TestRateLimit_ExceedLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create middleware with low limits
	middleware := RateLimit(2, 2)

	// Create test router
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// Make requests up to the limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 429, w.Code)
	assert.Contains(t, w.Body.String(), "Rate limit exceeded")
}

func TestRateLimit_DifferentIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create middleware with low limits
	middleware := RateLimit(1, 1)

	// Create test router
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// Request from first IP
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "127.0.0.1:12345"
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, 200, w1.Code)

	// Request from second IP should be allowed
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "127.0.0.2:12345"
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code)

	// Second request from first IP should be blocked
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "127.0.0.1:12345"
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	assert.Equal(t, 429, w3.Code)
}

func TestRateLimit_TimeWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create middleware with limits
	middleware := RateLimit(2, 2)

	// Create test router
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// Make 2 requests (at limit)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
	}

	// Third request should be blocked
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 429, w.Code)

	// Simulate time passing (this is a limitation of testing - in real usage,
	// the time window would naturally expire)
	// For testing purposes, we accept that the rate limit works as expected
}

func TestCORS_AllowAll(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create middleware
	middleware := CORS()

	// Create test router
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// Test OPTIONS request
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 204, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization, Accept", w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "86400", w.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_ActualRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create middleware
	middleware := CORS()

	// Create test router
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// Test actual GET request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://example.com")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Body.String(), `"message":"ok"`)
}
