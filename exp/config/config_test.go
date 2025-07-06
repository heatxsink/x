package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFromFile(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.txt")
		expectedContent := "test configuration content"

		err := os.WriteFile(testFile, []byte(expectedContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		content, err := FromFile(testFile)
		if err != nil {
			t.Errorf("FromFile failed: %v", err)
		}

		if string(content) != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, string(content))
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		content, err := FromFile("/nonexistent/file.txt")
		if err == nil {
			t.Error("Expected error for nonexistent file, got nil")
		}

		if content != nil {
			t.Errorf("Expected nil content for nonexistent file, got %v", content)
		}

		if !strings.Contains(err.Error(), "no such file or directory") && !strings.Contains(err.Error(), "cannot find the file") {
			t.Errorf("Expected file not found error, got: %v", err)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "empty.txt")

		err := os.WriteFile(testFile, []byte(""), 0644)
		if err != nil {
			t.Fatalf("Failed to create empty test file: %v", err)
		}

		content, err := FromFile(testFile)
		if err != nil {
			t.Errorf("FromFile failed for empty file: %v", err)
		}

		if len(content) != 0 {
			t.Errorf("Expected empty content, got %d bytes", len(content))
		}
	})

	t.Run("large file", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "large.txt")
		largeContent := strings.Repeat("test content line\n", 1000)

		err := os.WriteFile(testFile, []byte(largeContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create large test file: %v", err)
		}

		content, err := FromFile(testFile)
		if err != nil {
			t.Errorf("FromFile failed for large file: %v", err)
		}

		if string(content) != largeContent {
			t.Error("Large file content mismatch")
		}
	})

	t.Run("binary file", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "binary.dat")
		binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD}

		err := os.WriteFile(testFile, binaryContent, 0644)
		if err != nil {
			t.Fatalf("Failed to create binary test file: %v", err)
		}

		content, err := FromFile(testFile)
		if err != nil {
			t.Errorf("FromFile failed for binary file: %v", err)
		}

		if len(content) != len(binaryContent) {
			t.Errorf("Expected %d bytes, got %d bytes", len(binaryContent), len(content))
		}

		for i, b := range binaryContent {
			if content[i] != b {
				t.Errorf("Binary content mismatch at byte %d: expected %02x, got %02x", i, b, content[i])
			}
		}
	})
}

func TestFromURI(t *testing.T) {
	ctx := context.Background()

	t.Run("file URI", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "config.json")
		expectedContent := `{"key": "value"}`

		err := os.WriteFile(testFile, []byte(expectedContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		uri := "file://" + testFile
		content, err := FromURI(ctx, uri)
		if err != nil {
			t.Errorf("FromURI failed for file URI: %v", err)
		}

		if string(content) != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, string(content))
		}
	})

	t.Run("file URI with relative path", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "relative.txt")
		expectedContent := "relative path content"

		err := os.WriteFile(testFile, []byte(expectedContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Test with file:// prefix but relative-style path
		uri := "file:" + testFile
		content, err := FromURI(ctx, uri)
		if err != nil {
			t.Errorf("FromURI failed for relative file URI: %v", err)
		}

		if string(content) != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, string(content))
		}
	})

	t.Run("unsupported scheme", func(t *testing.T) {
		uri := "http://example.com/config.json"
		content, err := FromURI(ctx, uri)
		if err == nil {
			t.Error("Expected error for unsupported scheme, got nil")
		}

		if content != nil {
			t.Errorf("Expected nil content for unsupported scheme, got %v", content)
		}

		if !strings.Contains(err.Error(), "not supported") {
			t.Errorf("Expected 'not supported' error, got: %v", err)
		}
	})

	t.Run("invalid URI", func(t *testing.T) {
		uri := "://invalid-uri"
		content, err := FromURI(ctx, uri)
		if err == nil {
			t.Error("Expected error for invalid URI, got nil")
		}

		if content != nil {
			t.Errorf("Expected nil content for invalid URI, got %v", content)
		}
	})

	t.Run("GCS URI format", func(t *testing.T) {
		// Test that GCS URI parsing works (even though we can't test the actual GCS call without credentials)
		uri := "gs://test-bucket/config.json"
		_, err := FromURI(ctx, uri)

		// We expect this to fail with a GCS-related error, not a URI parsing error
		if err == nil {
			t.Error("Expected GCS error (no credentials), got nil")
		}

		// Should not be a URI parsing error
		if strings.Contains(err.Error(), "not supported") {
			t.Errorf("URI parsing failed for GCS scheme: %v", err)
		}
	})

	t.Run("Secret Manager URI format", func(t *testing.T) {
		// Test that Secret Manager URI parsing works
		uri := "secret://projects/test-project/secrets/test-secret/versions/latest"
		_, err := FromURI(ctx, uri)

		// We expect this to fail with a Secret Manager-related error, not a URI parsing error
		if err == nil {
			t.Error("Expected Secret Manager error (no credentials), got nil")
		}

		// Should not be a URI parsing error
		if strings.Contains(err.Error(), "not supported") {
			t.Errorf("URI parsing failed for secret scheme: %v", err)
		}
	})
}

func TestFromGCS(t *testing.T) {
	ctx := context.Background()

	t.Run("GCS authentication error", func(t *testing.T) {
		// This test will fail due to authentication, but verifies the function signature and error handling
		content, err := FromGCS(ctx, "test-bucket", "test-object")
		if err == nil {
			t.Error("Expected GCS authentication error, got nil")
		}

		if content != nil {
			t.Errorf("Expected nil content for GCS error, got %v", content)
		}

		// Error should be related to authentication or GCS client creation
		if !strings.Contains(err.Error(), "could not find default credentials") &&
			!strings.Contains(err.Error(), "Application Default Credentials") &&
			!strings.Contains(err.Error(), "authentication") {
			t.Logf("GCS error (expected): %v", err)
		}
	})

	t.Run("empty bucket name", func(t *testing.T) {
		content, err := FromGCS(ctx, "", "test-object")
		if err == nil {
			t.Error("Expected error for empty bucket name, got nil")
		}

		if content != nil {
			t.Errorf("Expected nil content for empty bucket, got %v", content)
		}
	})

	t.Run("empty object key", func(t *testing.T) {
		content, err := FromGCS(ctx, "test-bucket", "")
		if err == nil {
			t.Error("Expected error for empty object key, got nil")
		}

		if content != nil {
			t.Errorf("Expected nil content for empty object key, got %v", content)
		}
	})
}

func TestFromSecretManager(t *testing.T) {
	ctx := context.Background()

	t.Run("Secret Manager authentication error", func(t *testing.T) {
		// This test will fail due to authentication, but verifies the function signature and error handling
		resource := "projects/test-project/secrets/test-secret/versions/latest"
		content, err := FromSecretManager(ctx, resource)
		if err == nil {
			t.Error("Expected Secret Manager authentication error, got nil")
		}

		if content != nil {
			t.Errorf("Expected nil content for Secret Manager error, got %v", content)
		}

		// Error should mention failed to create client or authentication
		if !strings.Contains(err.Error(), "failed to create secret manager client") {
			t.Logf("Secret Manager error (expected): %v", err)
		}
	})

	t.Run("empty resource name", func(t *testing.T) {
		content, err := FromSecretManager(ctx, "")
		if err == nil {
			t.Error("Expected error for empty resource name, got nil")
		}

		if content != nil {
			t.Errorf("Expected nil content for empty resource, got %v", content)
		}
	})

	t.Run("invalid resource format", func(t *testing.T) {
		content, err := FromSecretManager(ctx, "invalid-resource-format")
		if err == nil {
			t.Error("Expected error for invalid resource format, got nil")
		}

		if content != nil {
			t.Errorf("Expected nil content for invalid resource, got %v", content)
		}
	})
}

func TestURISchemeHandling(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name          string
		uri           string
		expectError   bool
		errorContains string
	}{
		{
			name:          "file scheme",
			uri:           "file:///tmp/test.txt",
			expectError:   true, // file doesn't exist
			errorContains: "no such file",
		},
		{
			name:          "gs scheme",
			uri:           "gs://bucket/object",
			expectError:   true, // no credentials
			errorContains: "",   // GCS error varies
		},
		{
			name:          "secret scheme",
			uri:           "secret://projects/p/secrets/s/versions/v",
			expectError:   true, // no credentials
			errorContains: "",   // Error varies based on auth setup
		},
		{
			name:          "http scheme - unsupported",
			uri:           "http://example.com/config",
			expectError:   true,
			errorContains: "not supported",
		},
		{
			name:          "https scheme - unsupported",
			uri:           "https://example.com/config",
			expectError:   true,
			errorContains: "not supported",
		},
		{
			name:          "ftp scheme - unsupported",
			uri:           "ftp://example.com/config",
			expectError:   true,
			errorContains: "not supported",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := FromURI(ctx, tc.uri)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for URI '%s', got nil", tc.uri)
				}

				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tc.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for URI '%s': %v", tc.uri, err)
				}
			}

			if err != nil && content != nil {
				t.Errorf("Expected nil content when error occurred, got %v", content)
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Create multiple test files
	for i := 0; i < 5; i++ {
		testFile := filepath.Join(tempDir, fmt.Sprintf("test%d.txt", i))
		content := fmt.Sprintf("content for file %d", i)
		err := os.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}
	}

	// Test concurrent access to files
	done := make(chan error, 5)

	for i := 0; i < 5; i++ {
		go func(index int) {
			uri := fmt.Sprintf("file://%s", filepath.Join(tempDir, fmt.Sprintf("test%d.txt", index)))
			content, err := FromURI(ctx, uri)
			if err != nil {
				done <- err
				return
			}

			expected := fmt.Sprintf("content for file %d", index)
			if string(content) != expected {
				done <- fmt.Errorf("content mismatch for file %d", index)
				return
			}

			done <- nil
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent access failed: %v", err)
		}
	}
}

func TestEdgeCases(t *testing.T) {

	t.Run("file with special characters", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "file with spaces & symbols!.txt")
		expectedContent := "content with special chars: áéíóú"

		err := os.WriteFile(testFile, []byte(expectedContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		content, err := FromFile(testFile)
		if err != nil {
			t.Errorf("FromFile failed for file with special chars: %v", err)
		}

		if string(content) != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, string(content))
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// File operations should still work with cancelled context since they don't use it
		uri := "file://" + testFile
		content, err := FromURI(cancelCtx, uri)
		if err != nil {
			t.Errorf("FromURI failed with cancelled context: %v", err)
		}

		if string(content) != "test" {
			t.Errorf("Expected 'test', got '%s'", string(content))
		}
	})
}
