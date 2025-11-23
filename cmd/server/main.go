package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"qwen-go-proxy/internal/infrastructure/config"
	"qwen-go-proxy/internal/infrastructure/logging"
	"qwen-go-proxy/internal/infrastructure/middleware"
	"qwen-go-proxy/internal/infrastructure/repositories"
	"qwen-go-proxy/internal/infrastructure/services"
	"qwen-go-proxy/internal/interfaces/controllers"
	"qwen-go-proxy/internal/usecases/auth"
	"qwen-go-proxy/internal/usecases/proxy"
	"qwen-go-proxy/internal/usecases/streaming"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	startTime := time.Now()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger := logging.NewLoggerFromConfig(cfg)
	if logger == nil {
		log.Fatalf("Failed to initialize logger")
	}

	// Initialize infrastructure services (domain interfaces)
	oauthService := services.NewOAuthService(cfg.QWENOAuthBaseURL)
	aiService := services.NewAIService(cfg)

	// Initialize repository implementation (domain interface)
	credentialRepo := repositories.NewFileCredentialRepository(cfg.QWENDir)

	// Initialize use cases (application interfaces)
	authUseCase := auth.NewAuthUseCase(cfg, oauthService, credentialRepo, logger)
	streamingUseCase := streaming.NewStreamingUseCase(logger)
	proxyUseCase := proxy.NewProxyUseCase(authUseCase, aiService, streamingUseCase, logger, cfg.DefaultModel)

	// Initialize controllers
	apiController := controllers.NewAPIController(proxyUseCase, logger)

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create router
	router := chi.NewRouter()

	// Add middleware
	router.Use(middleware.RequestID())
	router.Use(middleware.RequestLogging(logger, cfg.DebugMode))
	router.Use(middleware.RateLimit(cfg.RateLimitRequestsPerSecond, cfg.RateLimitBurst))
	router.Use(middleware.CORS())

	// Add security headers middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
			next.ServeHTTP(w, r)
		})
	})

	// Root endpoint
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{"message": "Qwen API proxy is running"}
		json.NewEncoder(w).Encode(response)
	})

	// Health check endpoint (OpenAI compatible)
	router.Get("/health", apiController.OpenAIHealthHandler)

	// Enhanced health check with system metrics
	router.Get("/health/detailed", func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetRequestID(r.Context())
		w.Header().Set("X-Request-ID", requestID) // Ensure it's in response headers

		health := map[string]interface{}{
			"status":     "healthy",
			"timestamp":  time.Now().Unix(),
			"version":    "1.0.0",
			"uptime":     time.Since(startTime).String(),
			"request_id": requestID,
			"config": map[string]interface{}{
				"debug_mode":  cfg.DebugMode,
				"log_level":   cfg.LogLevel,
				"server_host": cfg.ServerHost,
				"server_port": cfg.ServerPort,
			},
		}

		// Check authentication status
		credentials, err := authUseCase.EnsureAuthenticated()
		if err != nil {
			logger.Warn("Health check authentication failed", "request_id", requestID, "error", err)
			health["auth_status"] = "unauthenticated"
			health["auth_error"] = err.Error()
		} else {
			health["auth_status"] = "authenticated"
			health["auth_info"] = credentials.Sanitize()
		}

		logger.Info("Health check requested", "request_id", requestID, "status", "healthy")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(health)
	})

	// Authentication endpoint
	router.Get("/auth", apiController.AuthenticateHandler)

	// OpenAI compatible endpoints
	router.Get("/v1/models", apiController.OpenAIModelsHandler)
	router.Post("/v1/completions", apiController.OpenAICompletionsHandler)
	router.Post("/v1/chat/completions", apiController.ChatCompletionsHandler)

	// Startup authentication check
	logger.Info("Starting Qwen Proxy")

	// Check if credentials exist (this is handled by the use case now)
	_, err = authUseCase.EnsureAuthenticated()
	if err != nil {
		logger.Info("No Qwen OAuth credentials found")
		logger.Info("The server will automatically handle OAuth2 device authentication when first accessed")
		logger.Info("Follow the prompts to authenticate with your Qwen account")
	} else {
		logger.Info("Qwen proxy is ready and authenticated")
	}

	logger.Info("Starting server", "host", cfg.ServerHost, "port", cfg.ServerPort)

	// Create HTTP server with Gin router
	srv := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	// Configure TLS if enabled
	if cfg.EnableTLS {
		if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
			log.Fatalf("TLS enabled but certificate files not configured")
		}

		logger.Info("TLS enabled", "cert_file", cfg.TLSCertFile, "key_file", cfg.TLSKeyFile)

		// In production, you might want to add more TLS configuration here
		// such as MinVersion, CipherSuites, etc.
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", "address", cfg.GetServerAddress())
		var err error
		if cfg.EnableTLS {
			err = srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			// Don't use log.Fatalf here as it would prevent graceful shutdown
			stop()
		}
	}()

	logger.Info("Server started successfully", "address", cfg.GetServerAddress())

	// Wait for interrupt signal
	<-ctx.Done()
	logger.Info("Shutting down server...")

	// Create a context with timeout for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// Gracefully shutdown the server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	} else {
		logger.Info("Server stopped gracefully")
	}

	// Cleanup resources
	logger.Info("Cleaning up resources...")
	// Add any cleanup logic here for gateways, repositories, etc.

	logger.Info("Shutdown complete")
}
