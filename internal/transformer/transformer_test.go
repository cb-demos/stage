package transformer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewCache(t *testing.T) {
	cache := NewCache()

	if cache == nil {
		t.Fatal("expected cache to be created")
	}

	if cache.files == nil {
		t.Error("expected files map to be initialized")
	}

	if cache.Size() != 0 {
		t.Errorf("expected empty cache, got size %d", cache.Size())
	}
}

func TestCacheOperations(t *testing.T) {
	cache := NewCache()

	// Test Set and Get
	testPath := "test.html"
	testContent := []byte("<html>test</html>")

	cache.Set(testPath, testContent)

	// Verify Size
	if cache.Size() != 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}

	// Verify Get
	content, exists := cache.Get(testPath)
	if !exists {
		t.Error("expected content to exist in cache")
	}

	if string(content) != string(testContent) {
		t.Errorf("expected content %s, got %s", testContent, content)
	}

	// Test Get for non-existent key
	_, exists = cache.Get("nonexistent.html")
	if exists {
		t.Error("expected content not to exist")
	}

	// Test multiple entries
	cache.Set("test2.js", []byte("console.log('test')"))
	if cache.Size() != 2 {
		t.Errorf("expected size 2, got %d", cache.Size())
	}
}

func TestTransform(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		replacements map[string]string
		expected     string
	}{
		{
			name:    "single replacement",
			content: "const key = '__FF_SDK_KEY__';",
			replacements: map[string]string{
				"FF_SDK_KEY": "test-123",
			},
			expected: "const key = 'test-123';",
		},
		{
			name:    "multiple replacements",
			content: "const key = '__FF_SDK_KEY__'; const endpoint = '__API_ENDPOINT__';",
			replacements: map[string]string{
				"FF_SDK_KEY":   "test-123",
				"API_ENDPOINT": "https://api.test.com",
			},
			expected: "const key = 'test-123'; const endpoint = 'https://api.test.com';",
		},
		{
			name:    "multiple occurrences",
			content: "__KEY__ and __KEY__ should both be replaced with __KEY__",
			replacements: map[string]string{
				"KEY": "VALUE",
			},
			expected: "VALUE and VALUE should both be replaced with VALUE",
		},
		{
			name:         "no replacements",
			content:      "const key = 'static-value';",
			replacements: map[string]string{},
			expected:     "const key = 'static-value';",
		},
		{
			name:    "placeholder not present",
			content: "const key = 'static-value';",
			replacements: map[string]string{
				"FF_SDK_KEY": "test-123",
			},
			expected: "const key = 'static-value';",
		},
		{
			name:    "HTML content",
			content: "<html><body><script>window.sdkKey = '__FF_SDK_KEY__';</script></body></html>",
			replacements: map[string]string{
				"FF_SDK_KEY": "prod-key-xyz",
			},
			expected: "<html><body><script>window.sdkKey = 'prod-key-xyz';</script></body></html>",
		},
		{
			name:    "case sensitive",
			content: "__FF_SDK_KEY__ __ff_sdk_key__",
			replacements: map[string]string{
				"FF_SDK_KEY": "replaced",
			},
			expected: "replaced __ff_sdk_key__",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := New("/tmp", tt.replacements)
			result := trans.transform([]byte(tt.content))

			if string(result) != tt.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, string(result))
			}
		})
	}
}

func TestShouldTransform(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		// Should transform
		{"index.html", true},
		{"app.js", true},
		{"module.mjs", true},
		{"component.jsx", true},
		{"types.ts", true},
		{"Component.tsx", true},
		{"styles.css", true},
		{"config.json", true},
		{"data.xml", true},
		{"icon.svg", true},
		{"readme.txt", true},
		{"docs.md", true},
		// Mixed case extensions
		{"file.HTML", true},
		{"file.JS", true},
		{"file.CSS", true},
		// Should NOT transform
		{"image.png", false},
		{"photo.jpg", false},
		{"photo.jpeg", false},
		{"animation.gif", false},
		{"image.webp", false},
		{"favicon.ico", false},
		{"font.woff", false},
		{"font.woff2", false},
		{"font.ttf", false},
		{"font.eot", false},
		{"data.bin", false},
		{"archive.zip", false},
		{"document.pdf", false},
		{"noextension", false},
		{".hidden", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := shouldTransform(tt.filename)
			if result != tt.expected {
				t.Errorf("for file %s, expected %v, got %v", tt.filename, tt.expected, result)
			}
		})
	}
}

func TestTransformAll(t *testing.T) {
	// Create a temporary directory structure with test files
	tempDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"index.html":     "<html><body>Key: __TEST_KEY__</body></html>",
		"app.js":         "const key = '__TEST_KEY__';",
		"styles.css":     ".class { content: '__TEST_KEY__'; }",
		"config.json":    "{\"key\": \"__TEST_KEY__\"}",
		"image.png":      "fake-png-data", // Should not be transformed
		"subdir/sub.js":  "const sub = '__TEST_KEY__';",
	}

	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", path, err)
		}
	}

	// Create transformer with replacements
	replacements := map[string]string{
		"TEST_KEY": "replaced-value",
	}
	trans := New(tempDir, replacements)

	// Run transformation
	err := trans.TransformAll()
	if err != nil {
		t.Fatalf("TransformAll failed: %v", err)
	}

	// Verify cache contains expected files
	expectedCached := []string{"index.html", "app.js", "styles.css", "config.json", "subdir/sub.js"}
	cache := trans.GetCache()

	if cache.Size() != len(expectedCached) {
		t.Errorf("expected %d cached files, got %d", len(expectedCached), cache.Size())
	}

	// Verify each file is transformed correctly
	for _, relPath := range expectedCached {
		content, exists := cache.Get(relPath)
		if !exists {
			t.Errorf("expected file %s to be in cache", relPath)
			continue
		}

		contentStr := string(content)
		if containsPlaceholder(contentStr) {
			t.Errorf("file %s still contains placeholder: %s", relPath, contentStr)
		}

		if !containsReplacement(contentStr) {
			t.Errorf("file %s doesn't contain replacement value: %s", relPath, contentStr)
		}
	}

	// Verify image.png is NOT in cache
	_, exists := cache.Get("image.png")
	if exists {
		t.Error("expected image.png not to be in cache")
	}

	// Verify original files are unchanged
	originalContent, _ := os.ReadFile(filepath.Join(tempDir, "index.html"))
	if !containsPlaceholder(string(originalContent)) {
		t.Error("original file should still contain placeholder")
	}
}

func TestTransformAllWithNoReplacements(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tempDir, "test.html")
	os.WriteFile(testFile, []byte("<html>test</html>"), 0644)

	// Create transformer with no replacements
	trans := New(tempDir, map[string]string{})

	// Should not error, just log a warning and return early
	err := trans.TransformAll()
	if err != nil {
		t.Errorf("TransformAll should not error with no replacements: %v", err)
	}

	// Cache should be empty since TransformAll returns early when no replacements exist
	if trans.GetCache().Size() != 0 {
		t.Errorf("expected empty cache when no replacements configured, got %d items", trans.GetCache().Size())
	}
}

func TestTransformAllWithEmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()

	replacements := map[string]string{
		"TEST_KEY": "value",
	}
	trans := New(tempDir, replacements)

	err := trans.TransformAll()
	if err != nil {
		t.Errorf("TransformAll should not error with empty directory: %v", err)
	}

	if trans.GetCache().Size() != 0 {
		t.Errorf("expected empty cache, got %d items", trans.GetCache().Size())
	}
}

func TestTransformAllWithNonexistentDirectory(t *testing.T) {
	replacements := map[string]string{
		"TEST_KEY": "value",
	}
	trans := New("/nonexistent/directory", replacements)

	err := trans.TransformAll()
	if err == nil {
		t.Error("expected error with nonexistent directory")
	}
}

func TestCacheConcurrency(t *testing.T) {
	cache := NewCache()

	// Start multiple goroutines doing concurrent operations
	const numGoroutines = 100
	const numOperations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("file-%d-%d.html", id, j)
				value := []byte(fmt.Sprintf("content-%d-%d", id, j))

				// Write
				cache.Set(key, value)

				// Read
				if content, exists := cache.Get(key); !exists {
					t.Errorf("expected key %s to exist", key)
				} else if string(content) != string(value) {
					t.Errorf("content mismatch for key %s", key)
				}

				// Check size (triggers read lock)
				_ = cache.Size()

				// Check stats (triggers read lock)
				_, _, _ = cache.Stats()
			}
		}(i)
	}

	wg.Wait()

	expectedSize := numGoroutines * numOperations
	if cache.Size() != expectedSize {
		t.Errorf("expected cache size %d, got %d", expectedSize, cache.Size())
	}

	// Verify stats are tracked
	hits, misses, sizeBytes := cache.Stats()
	if hits == 0 {
		t.Error("expected some cache hits")
	}
	if sizeBytes == 0 {
		t.Error("expected non-zero cache size in bytes")
	}
	t.Logf("Cache stats: hits=%d, misses=%d, sizeBytes=%d", hits, misses, sizeBytes)
}

func TestCacheStats(t *testing.T) {
	cache := NewCache()

	// Initially, stats should be zero
	hits, misses, sizeBytes := cache.Stats()
	if hits != 0 || misses != 0 || sizeBytes != 0 {
		t.Errorf("expected zero stats for empty cache, got hits=%d, misses=%d, bytes=%d", hits, misses, sizeBytes)
	}

	// Add some data
	cache.Set("test1.html", []byte("content1"))
	cache.Set("test2.js", []byte("content2longer"))

	// Get existing file (hit)
	cache.Get("test1.html")

	// Get non-existing file (miss)
	cache.Get("nonexistent.html")

	// Check stats
	hits, misses, sizeBytes = cache.Stats()
	if hits != 1 {
		t.Errorf("expected 1 hit, got %d", hits)
	}
	if misses != 1 {
		t.Errorf("expected 1 miss, got %d", misses)
	}

	expectedBytes := len("content1") + len("content2longer")
	if sizeBytes != expectedBytes {
		t.Errorf("expected %d bytes, got %d", expectedBytes, sizeBytes)
	}
}

// Helper functions

func containsPlaceholder(content string) bool {
	return strings.Contains(content, "__TEST_KEY__")
}

func containsReplacement(content string) bool {
	return strings.Contains(content, "replaced-value")
}
