package transformer

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// Cache stores transformed file contents in memory
type Cache struct {
	mu     sync.RWMutex
	files  map[string][]byte // map of file path -> transformed content
	hits   uint64            // cache hit counter
	misses uint64            // cache miss counter
}

// NewCache creates a new cache instance
func NewCache() *Cache {
	return &Cache{
		files: make(map[string][]byte),
	}
}

// Get retrieves transformed content from cache
func (c *Cache) Get(path string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	content, exists := c.files[path]

	if exists {
		atomic.AddUint64(&c.hits, 1)
	} else {
		atomic.AddUint64(&c.misses, 1)
	}

	return content, exists
}

// Set stores transformed content in cache
func (c *Cache) Set(path string, content []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files[path] = content
}

// Size returns the number of cached files
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.files)
}

// Stats returns cache statistics
func (c *Cache) Stats() (hits, misses uint64, sizeBytes int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, content := range c.files {
		sizeBytes += len(content)
	}

	return atomic.LoadUint64(&c.hits), atomic.LoadUint64(&c.misses), sizeBytes
}

// Transformer handles asset transformation
type Transformer struct {
	assetDir     string
	replacements map[string]string
	cache        *Cache
}

// New creates a new Transformer instance
func New(assetDir string, replacements map[string]string) *Transformer {
	return &Transformer{
		assetDir:     assetDir,
		replacements: replacements,
		cache:        NewCache(),
	}
}

// TransformAll scans the asset directory and transforms all applicable files
func (t *Transformer) TransformAll() error {
	slog.Info("Starting asset transformation", "assetDir", t.assetDir, "replacements", len(t.replacements))

	if len(t.replacements) == 0 {
		slog.Warn("No STAGE_* environment variables found, no transformations will be applied")
		return nil
	}

	transformCount := 0
	err := filepath.WalkDir(t.assetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only transform text-based files that might contain placeholders
		if !shouldTransform(path) {
			return nil
		}

		// Read the file
		content, err := os.ReadFile(path)
		if err != nil {
			slog.Error("Failed to read file, skipping", "path", path, "error", err)
			return nil // Continue with other files
		}

		// Apply transformations
		transformed := t.transform(content)

		// Store in cache (using relative path from asset directory)
		relPath, err := filepath.Rel(t.assetDir, path)
		if err != nil {
			slog.Error("Failed to get relative path", "path", path, "error", err)
			return err
		}

		// Normalize path separators for cross-platform compatibility
		relPath = filepath.ToSlash(relPath)

		t.cache.Set(relPath, transformed)
		transformCount++

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to transform assets: %w", err)
	}

	// Get cache statistics and warn if cache is large
	_, _, sizeBytes := t.cache.Stats()
	sizeMB := sizeBytes / (1024 * 1024)

	slog.Info("Asset transformation complete", "filesTransformed", transformCount, "cachedFiles", t.cache.Size(), "cacheSizeMB", sizeMB)

	const warnThresholdMB = 100
	if sizeMB > warnThresholdMB {
		slog.Warn("Cache size is large, consider reviewing asset directory size", "cacheSizeMB", sizeMB, "thresholdMB", warnThresholdMB)
	}

	return nil
}

// transform applies string replacements to content
func (t *Transformer) transform(content []byte) []byte {
	contentStr := string(content)

	// Apply each replacement
	for placeholder, value := range t.replacements {
		// Create the full placeholder pattern: __PLACEHOLDER__
		pattern := fmt.Sprintf("__%s__", placeholder)
		contentStr = strings.ReplaceAll(contentStr, pattern, value)
	}

	return []byte(contentStr)
}

// GetCache returns the transformation cache
func (t *Transformer) GetCache() *Cache {
	return t.cache
}

// shouldTransform determines if a file should be transformed based on extension
func shouldTransform(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	// List of file extensions that might contain placeholders
	transformableExts := map[string]bool{
		".html": true,
		".htm":  true,
		".js":   true,
		".mjs":  true,
		".jsx":  true,
		".ts":   true,
		".tsx":  true,
		".css":  true,
		".json": true,
		".xml":  true,
		".svg":  true,
		".txt":  true,
		".md":   true,
		".env":  true,
		".yml":  true,
		".yaml": true,
	}

	return transformableExts[ext]
}
