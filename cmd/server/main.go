package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"qwen-go-proxy/internal/infrastructure/config"
	"qwen-go-proxy/internal/infrastructure/logging"
	"qwen-go-proxy/internal/infrastructure/middleware"
	"qwen-go-proxy/internal/interfaces/controllers"
	"qwen-go-proxy/internal/interfaces/gateways"
	"qwen-go-proxy/internal/usecases/auth"
	"qwen-go-proxy/internal/usecases/proxy"
	"qwen-go-proxy/internal/usecases/streaming"
)

func main() {
	startTime := time.Now()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Initialize logger
	logger := logging.NewLoggerFromConfig(cfg)
	if logger == nil {
		log.Fatalf("Failed to initialize logger")
	}

	// Initialize gateways
	oauthGateway := gateways.NewOAuthGateway(cfg.QWENOAuthBaseURL)
	qwenGateway := gateways.NewQwenAPIGateway(cfg)

	// Initialize repositories
	credentialRepo := auth.NewFileCredentialRepository(cfg.QWENDir)

	// Initialize use cases
	authUseCase := auth.NewAuthUseCase(cfg, oauthGateway, credentialRepo, logger)
	streamingUseCase := streaming.NewStreamingUseCase(logger)
	proxyUseCase := proxy.NewProxyUseCase(authUseCase, qwenGateway, streamingUseCase, logger)

	// Initialize controllers
	apiController := controllers.NewAPIController(proxyUseCase, logger)

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create router
	router := gin.Default()

	// Add middleware
	router.Use(middleware.RequestID())
	router.Use(middleware.RequestLogging(logger, cfg.DebugMode))
	router.Use(middleware.RateLimit(cfg.RateLimitRequestsPerSecond, cfg.RateLimitBurst))
	router.Use(middleware.CORS())

	// Add security headers middleware
	router.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000")
		c.Next()
	})

	// Root endpoint
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Qwen API proxy is running"})
	})

	// Health check endpoint (OpenAI compatible)
	router.GET("/health", apiController.OpenAIHealthHandler)

	// Enhanced health check with system metrics
	router.GET("/health/detailed", func(c *gin.Context) {
		requestID := middleware.GetRequestID(c)
		c.Header("X-Request-ID", requestID) // Ensure it's in response headers

		health := gin.H{
			"status":     "healthy",
			"timestamp":  time.Now().Unix(),
			"version":    "1.0.0",
			"uptime":     time.Since(startTime).String(),
			"request_id": requestID,
			"config": gin.H{
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
		c.JSON(http.StatusOK, health)
	})

	// Authentication endpoint
	router.GET("/auth", apiController.AuthenticateHandler)

	// OpenAI compatible endpoints
	router.GET("/v1/models", apiController.OpenAIModelsHandler)
	router.POST("/v1/completions", apiController.OpenAICompletionsHandler)
	router.POST("/v1/chat/completions", apiController.ChatCompletionsHandler)

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
