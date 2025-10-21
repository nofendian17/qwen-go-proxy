package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qwen-go-proxy/internal/infrastructure/logging"

	"github.com/stretchr/testify/assert"
)

func TestRequestID(t *testing.T) {
	// Create a handler that will be wrapped by the middleware
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		assert.NotEmpty(t, requestID)
		w.WriteHeader(http.StatusOK)
	})

	// Wrap the handler with the RequestID middleware
	handler := RequestID()(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2) // Should generate different IDs
}

func TestGetRequestID(t *testing.T) {
	// Test with request ID in context
	ctx := context.WithValue(context.Background(), RequestIDKey, "test-request-id")
	requestID := GetRequestID(ctx)
	assert.Equal(t, "test-request-id", requestID)

	// Test with no request ID in context (should return unknown)
	ctx = context.Background()
	requestID = GetRequestID(ctx)
	assert.Equal(t, "unknown", requestID)
}

func TestResponseWriterWrapper_Hijack(t *testing.T) {
	// Create a test response recorder
	rec := httptest.NewRecorder()

	// Create a responseWriterWrapper
	wrapper := &responseWriterWrapper{
		ResponseWriter: rec,
		body:           bytes.NewBuffer([]byte{}),
		status:         200,
	}

	// Try to hijack - this should fail since httptest.ResponseRecorder doesn't implement Hijacker
	_, _, err := wrapper.Hijack()

	// The error is expected since ResponseRecorder doesn't implement Hijacker
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not implement http.Hijacker")
}

func TestResponseWriterWrapper_Write(t *testing.T) {
	rec := httptest.NewRecorder()

	wrapper := &responseWriterWrapper{
		ResponseWriter: rec,
		body:           bytes.NewBuffer([]byte{}),
		status:         200,
	}

	data := []byte("test response")
	n, err := wrapper.Write(data)

	assert.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Equal(t, string(data), rec.Body.String())
	assert.Equal(t, string(data), wrapper.body.String())
}

func TestResponseWriterWrapper_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()

	wrapper := &responseWriterWrapper{
		ResponseWriter: rec,
		body:           bytes.NewBuffer([]byte{}),
		status:         200,
	}

	statusCode := http.StatusCreated
	wrapper.WriteHeader(statusCode)

	assert.Equal(t, statusCode, wrapper.status)
	assert.Equal(t, statusCode, rec.Code)
}

func TestRequestLogging_NonDebugMode(t *testing.T) {
	logger := logging.NewLogger("info")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	handler := RequestLogging(&logging.Logger{Logger: logger}, false)(nextHandler) // debugMode = false

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "response", rec.Body.String())
}

func TestRequestLogging_DebugMode(t *testing.T) {
	logger := logging.NewLogger("debug")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	handler := RequestLogging(&logging.Logger{Logger: logger}, true)(nextHandler) // debugMode = true

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "response", rec.Body.String())
}

func TestRequestLogging_WithRequestBody(t *testing.T) {
	logger := logging.NewLogger("debug")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, "request body", string(body))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	handler := RequestLogging(&logging.Logger{Logger: logger}, true)(nextHandler)

	req := httptest.NewRequest("POST", "/test", strings.NewReader("request body"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "response", rec.Body.String())
}

func TestRateLimit_AllowRequests(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	handler := RateLimit(10, 20)(nextHandler) // 10 requests per second, burst 20

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "response", rec.Body.String())

	// Check rate limit headers
	assert.NotEmpty(t, rec.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, rec.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, rec.Header().Get("X-RateLimit-Reset"))
}

func TestRateLimit_ExceedLimit(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	// Limit to 1 request per second
	handler := RateLimit(1, 1)(nextHandler)

	// Make first request - should be allowed
	req1 := httptest.NewRequest("GET", "/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Make second request immediately - should be rate limited
	req2 := httptest.NewRequest("GET", "/test", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)

	// Check if rate limit response has proper content
	assert.Contains(t, rec2.Body.String(), "Rate limit exceeded")
}

func TestCORS(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	handler := CORS()(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "response", rec.Body.String())

	// Check CORS headers
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization, Accept", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "86400", rec.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_OptionsRequest(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	handler := CORS()(nextHandler)

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestGetClientIP(t *testing.T) {
	// Test with X-Forwarded-For header
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")
	ip1 := getClientIP(req1)
	assert.Equal(t, "203.0.113.195", ip1)

	// Test with X-Real-IP header
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("X-Real-IP", "192.0.2.1")
	ip2 := getClientIP(req2)
	assert.Equal(t, "192.0.2.1", ip2)

	// Test with RemoteAddr
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.RemoteAddr = "192.0.2.4:12345"
	ip3 := getClientIP(req3)
	assert.Equal(t, "192.0.2.4", ip3)
}
