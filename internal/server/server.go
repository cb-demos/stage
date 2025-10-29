package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cb-demos/stage/internal/config"
	"github.com/cb-demos/stage/internal/transformer"
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	router      *gin.Engine
	config      *config.Config
	cache       *transformer.Cache
	httpServer  *http.Server
}

// New creates a new Server instance
func New(cfg *config.Config, cache *transformer.Cache) *Server {
	// Set Gin mode based on environment
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	s := &Server{
		router: router,
		config: cfg,
		cache:  cache,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/health", s.handleHealth)

	// Serve all other requests through the asset handler
	s.router.NoRoute(s.handleAssets)
}

// handleHealth returns server health status
func (s *Server) handleHealth(c *gin.Context) {
	hits, misses, sizeBytes := s.cache.Stats()
	c.JSON(http.StatusOK, gin.H{
		"status":       "ok",
		"cache_files":  s.cache.Size(),
		"cache_bytes":  sizeBytes,
		"cache_hits":   hits,
		"cache_misses": misses,
	})
}

// handleAssets serves static assets with transformation support
func (s *Server) handleAssets(c *gin.Context) {
	requestPath := c.Request.URL.Path

	// Remove leading slash for file system operations
	cleanPath := strings.TrimPrefix(requestPath, "/")

	// Clean the path to normalize it
	cleanPath = filepath.Clean(cleanPath)

	// Try to serve from cache first
	if content, exists := s.cache.Get(cleanPath); exists {
		slog.Debug("Serving from cache", "path", requestPath)
		s.serveContent(c, cleanPath, content)
		return
	}

	// Build full file system path
	fullPath := filepath.Join(s.config.AssetDir, cleanPath)

	// Verify the resolved path is within the asset directory (defense in depth)
	absAssetDir, err := filepath.Abs(s.config.AssetDir)
	if err != nil {
		slog.Error("Failed to resolve asset directory", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		slog.Error("Failed to resolve request path", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}

	if !strings.HasPrefix(absFullPath, absAssetDir) {
		slog.Warn("Path outside asset directory detected", "path", requestPath, "resolved", absFullPath)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "forbidden",
		})
		return
	}

	// Check if file exists
	fileInfo, err := os.Stat(fullPath)
	if err == nil && !fileInfo.IsDir() {
		// File exists but not in cache (e.g., images, fonts)
		slog.Debug("Serving original file", "path", requestPath)
		c.File(fullPath)
		return
	}

	// For SPA support: if path doesn't exist and should fallback to index.html
	if shouldFallbackToSPA(requestPath) {
		indexPath := "index.html"

		// Try cached index.html first
		if content, exists := s.cache.Get(indexPath); exists {
			slog.Debug("Serving index.html from cache for SPA route", "requestPath", requestPath)
			s.serveContent(c, indexPath, content)
			return
		}

		// Try original index.html
		indexFullPath := filepath.Join(s.config.AssetDir, indexPath)
		if _, err := os.Stat(indexFullPath); err == nil {
			slog.Debug("Serving original index.html for SPA route", "requestPath", requestPath)
			c.File(indexFullPath)
			return
		}
	}

	// Nothing found, return 404
	c.JSON(http.StatusNotFound, gin.H{
		"error": "not found",
		"path":  requestPath,
	})
}

// serveContent serves content with appropriate content type
func (s *Server) serveContent(c *gin.Context, path string, content []byte) {
	// Determine content type based on file extension
	contentType := getContentType(path)
	c.Data(http.StatusOK, contentType, content)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)
	slog.Info("Starting server", "address", addr)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// getContentType determines the MIME type based on file extension
func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	contentTypes := map[string]string{
		".html": "text/html; charset=utf-8",
		".htm":  "text/html; charset=utf-8",
		".css":  "text/css; charset=utf-8",
		".js":   "application/javascript; charset=utf-8",
		".mjs":  "application/javascript; charset=utf-8",
		".json": "application/json; charset=utf-8",
		".xml":  "application/xml; charset=utf-8",
		".svg":  "image/svg+xml",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".ico":  "image/x-icon",
		".woff": "font/woff",
		".woff2": "font/woff2",
		".ttf":  "font/ttf",
		".eot":  "application/vnd.ms-fontobject",
		".txt":  "text/plain; charset=utf-8",
		".md":   "text/markdown; charset=utf-8",
	}

	if ct, exists := contentTypes[ext]; exists {
		return ct
	}

	return "application/octet-stream"
}

// shouldFallbackToSPA determines if a request should fallback to serving index.html
func shouldFallbackToSPA(path string) bool {
	// Don't fallback for API routes
	if strings.HasPrefix(path, "/api/") {
		return false
	}

	// Don't fallback for special paths
	specialPaths := []string{
		"/.well-known/",
		"/metrics",
		"/health",
	}
	for _, sp := range specialPaths {
		if strings.HasPrefix(path, sp) {
			return false
		}
	}

	// Don't fallback if has file extension (likely a real file request)
	return filepath.Ext(path) == ""
}
