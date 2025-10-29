package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cb-demos/stage/internal/config"
	"github.com/cb-demos/stage/internal/server"
	"github.com/cb-demos/stage/internal/transformer"
)

func main() {
	// Configure structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: getLogLevel(),
	}))
	slog.SetDefault(logger)

	slog.Info("Starting stage - intelligent web server")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("Configuration loaded",
		"port", cfg.Port,
		"assetDir", cfg.AssetDir,
		"fmKeyConfigured", cfg.FMKey != "",
		"replacementCount", len(cfg.Replacements),
		"prometheusEnabled", cfg.PrometheusEnabled,
		"prometheusScenario", cfg.PrometheusScenario)

	// Create transformer and run transformations
	trans := transformer.New(cfg.AssetDir, cfg.Replacements)
	if err := trans.TransformAll(); err != nil {
		slog.Error("Failed to transform assets", "error", err)
		os.Exit(1)
	}

	// Create and start server
	srv := server.New(cfg, trans.GetCache(), logger)

	// Setup graceful shutdown
	go func() {
		if err := srv.Start(); err != nil {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	// Perform graceful shutdown with context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server stopped")
}

// getLogLevel returns the log level based on environment variable
func getLogLevel() slog.Level {
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "DEBUG", "debug":
		return slog.LevelDebug
	case "INFO", "info":
		return slog.LevelInfo
	case "WARN", "warn":
		return slog.LevelWarn
	case "ERROR", "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
