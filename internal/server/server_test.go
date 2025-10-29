package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cb-demos/stage/internal/config"
	"github.com/cb-demos/stage/internal/transformer"
)

func TestHealthEndpoint(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Port:         "8080",
		AssetDir:     tempDir,
		Host:         "0.0.0.0",
		Replacements: map[string]string{},
	}

	cache := transformer.NewCache()
	cache.Set("test.html", []byte("content"))

	srv := New(cfg, cache)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", response["status"])
	}

	// Cache files should be 1 since we added one item
	if cacheFiles, ok := response["cache_files"].(float64); !ok || int(cacheFiles) != 1 {
		t.Errorf("expected cache_files 1, got %v", response["cache_files"])
	}

	// Cache bytes should match content length
	if cacheBytes, ok := response["cache_bytes"].(float64); !ok || int(cacheBytes) != len("content") {
		t.Errorf("expected cache_bytes %d, got %v", len("content"), response["cache_bytes"])
	}

	// Should have hits and misses fields
	if _, ok := response["cache_hits"]; !ok {
		t.Error("expected cache_hits field in response")
	}

	if _, ok := response["cache_misses"]; !ok {
		t.Error("expected cache_misses field in response")
	}
}

func TestServeCachedAsset(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Port:         "8080",
		AssetDir:     tempDir,
		Host:         "0.0.0.0",
		Replacements: map[string]string{},
	}

	cache := transformer.NewCache()
	transformedContent := []byte("<html><body>Transformed Content</body></html>")
	cache.Set("index.html", transformedContent)

	srv := New(cfg, cache)

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != string(transformedContent) {
		t.Errorf("expected transformed content, got %s", w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("expected content type 'text/html; charset=utf-8', got %s", contentType)
	}
}

func TestServeOriginalAsset(t *testing.T) {
	tempDir := t.TempDir()

	// Create an image file (not in cache)
	imagePath := filepath.Join(tempDir, "logo.png")
	imageContent := []byte("fake-png-data")
	if err := os.WriteFile(imagePath, imageContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := &config.Config{
		Port:         "8080",
		AssetDir:     tempDir,
		Host:         "0.0.0.0",
		Replacements: map[string]string{},
	}

	cache := transformer.NewCache()
	srv := New(cfg, cache)

	req := httptest.NewRequest(http.MethodGet, "/logo.png", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != string(imageContent) {
		t.Errorf("expected original content, got %s", w.Body.String())
	}
}

func TestSPARouting(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Port:         "8080",
		AssetDir:     tempDir,
		Host:         "0.0.0.0",
		Replacements: map[string]string{},
	}

	cache := transformer.NewCache()
	indexContent := []byte("<html><body>SPA Index</body></html>")
	cache.Set("index.html", indexContent)

	srv := New(cfg, cache)

	// Test various SPA routes that should return index.html
	spaRoutes := []string{
		"/dashboard",
		"/users/123",
		"/settings/profile",
		"/about",
	}

	for _, route := range spaRoutes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			w := httptest.NewRecorder()

			srv.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200 for SPA route %s, got %d", route, w.Code)
			}

			if w.Body.String() != string(indexContent) {
				t.Errorf("expected index.html content for route %s, got %s", route, w.Body.String())
			}
		})
	}
}

func TestSPARoutingWithOriginalIndex(t *testing.T) {
	tempDir := t.TempDir()

	// Create index.html on disk (not in cache)
	indexPath := filepath.Join(tempDir, "index.html")
	indexContent := []byte("<html><body>Original Index</body></html>")
	if err := os.WriteFile(indexPath, indexContent, 0644); err != nil {
		t.Fatalf("failed to create index.html: %v", err)
	}

	cfg := &config.Config{
		Port:         "8080",
		AssetDir:     tempDir,
		Host:         "0.0.0.0",
		Replacements: map[string]string{},
	}

	cache := transformer.NewCache()
	srv := New(cfg, cache)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != string(indexContent) {
		t.Errorf("expected original index.html content, got %s", w.Body.String())
	}
}

func TestAPIRoutesNotSPARouted(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Port:         "8080",
		AssetDir:     tempDir,
		Host:         "0.0.0.0",
		Replacements: map[string]string{},
	}

	cache := transformer.NewCache()
	srv := New(cfg, cache)

	// API routes should return 404, not index.html
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for API route, got %d", w.Code)
	}
}

func TestFileWithExtensionNotFoundReturns404(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Port:         "8080",
		AssetDir:     tempDir,
		Host:         "0.0.0.0",
		Replacements: map[string]string{},
	}

	cache := transformer.NewCache()
	srv := New(cfg, cache)

	// Files with extensions that don't exist should return 404
	req := httptest.NewRequest(http.MethodGet, "/nonexistent.js", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func Test404Response(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Port:         "8080",
		AssetDir:     tempDir,
		Host:         "0.0.0.0",
		Replacements: map[string]string{},
	}

	cache := transformer.NewCache()
	srv := New(cfg, cache)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse 404 response: %v", err)
	}

	if response["error"] != "not found" {
		t.Errorf("expected error 'not found', got %v", response["error"])
	}
}

func TestGetContentType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"index.html", "text/html; charset=utf-8"},
		{"app.js", "application/javascript; charset=utf-8"},
		{"module.mjs", "application/javascript; charset=utf-8"},
		{"styles.css", "text/css; charset=utf-8"},
		{"data.json", "application/json; charset=utf-8"},
		{"config.xml", "application/xml; charset=utf-8"},
		{"icon.svg", "image/svg+xml"},
		{"image.png", "image/png"},
		{"photo.jpg", "image/jpeg"},
		{"photo.jpeg", "image/jpeg"},
		{"animation.gif", "image/gif"},
		{"image.webp", "image/webp"},
		{"favicon.ico", "image/x-icon"},
		{"font.woff", "font/woff"},
		{"font.woff2", "font/woff2"},
		{"font.ttf", "font/ttf"},
		{"font.eot", "application/vnd.ms-fontobject"},
		{"readme.txt", "text/plain; charset=utf-8"},
		{"docs.md", "text/markdown; charset=utf-8"},
		{"unknown.xyz", "application/octet-stream"},
		{"noextension", "application/octet-stream"},
		// Test case insensitivity
		{"FILE.HTML", "text/html; charset=utf-8"},
		{"FILE.JS", "application/javascript; charset=utf-8"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := getContentType(tt.path)
			if result != tt.expected {
				t.Errorf("for path %s, expected content type %s, got %s", tt.path, tt.expected, result)
			}
		})
	}
}

func TestServeNestedAssets(t *testing.T) {
	tempDir := t.TempDir()

	// Create nested directory structure
	nestedDir := filepath.Join(tempDir, "assets", "js")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}

	nestedFile := filepath.Join(nestedDir, "app.js")
	content := []byte("console.log('nested');")
	if err := os.WriteFile(nestedFile, content, 0644); err != nil {
		t.Fatalf("failed to create nested file: %v", err)
	}

	cfg := &config.Config{
		Port:         "8080",
		AssetDir:     tempDir,
		Host:         "0.0.0.0",
		Replacements: map[string]string{},
	}

	cache := transformer.NewCache()
	srv := New(cfg, cache)

	req := httptest.NewRequest(http.MethodGet, "/assets/js/app.js", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Body.String() != string(content) {
		t.Errorf("expected nested file content, got %s", w.Body.String())
	}
}

func TestCachePriority(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file on disk
	filePath := filepath.Join(tempDir, "test.html")
	originalContent := []byte("<html>original</html>")
	if err := os.WriteFile(filePath, originalContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := &config.Config{
		Port:         "8080",
		AssetDir:     tempDir,
		Host:         "0.0.0.0",
		Replacements: map[string]string{},
	}

	cache := transformer.NewCache()
	// Put transformed version in cache
	transformedContent := []byte("<html>transformed</html>")
	cache.Set("test.html", transformedContent)

	srv := New(cfg, cache)

	req := httptest.NewRequest(http.MethodGet, "/test.html", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Should serve from cache, not original file
	if w.Body.String() != string(transformedContent) {
		t.Errorf("expected transformed content from cache, got %s", w.Body.String())
	}

	if w.Body.String() == string(originalContent) {
		t.Error("served original content instead of cached content")
	}
}

func TestPathTraversalPrevention(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file in the temp directory
	testFile := filepath.Join(tempDir, "test.html")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := &config.Config{
		Port:         "8080",
		AssetDir:     tempDir,
		Host:         "0.0.0.0",
		Replacements: map[string]string{},
	}

	cache := transformer.NewCache()
	srv := New(cfg, cache)

	// Test various path traversal attempts
	traversalAttempts := []struct {
		name string
		path string
	}{
		{"double dot parent", "/../etc/passwd"},
		{"multiple double dots", "/../../etc/passwd"},
		{"nested traversal", "/foo/../../../etc/passwd"},
		{"absolute path escape", "/../../../../../etc/passwd"},
		{"current and parent dirs", "/./../../etc/passwd"},
	}

	for _, tt := range traversalAttempts {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			srv.router.ServeHTTP(w, req)

			// Should return either 403 Forbidden or 404 Not Found
			// (403 if the path resolution detects the escape, 404 if the file doesn't exist after escaping asset dir)
			if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
				t.Errorf("expected status 403 or 404 for path traversal attempt, got %d for path %s", w.Code, tt.path)
			}

			// Ensure we didn't serve sensitive content
			body := w.Body.String()
			if strings.Contains(body, "root:") || strings.Contains(body, "/bin/") {
				t.Errorf("path traversal attack succeeded! Served sensitive content for path %s: %s", tt.path, body)
			}
		})
	}
}
