package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppIDValidation(t *testing.T) {
	validIDs := []string{"myapp", "app-123", "backend_api", "node-v20-srv"}
	for _, id := range validIDs {
		if err := ValidateAppID(id); err != nil {
			t.Errorf("Expected valid app ID %s, got error: %v", id, err)
		}
	}

	invalidIDs := []string{"", "../escaped", "app/sub", "app;rm -rf", "name with space"}
	for _, id := range invalidIDs {
		if err := ValidateAppID(id); err == nil {
			t.Errorf("Expected error for invalid app ID %s, got nil", id)
		}
	}
}

func TestPathTraversalPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runpilot-security-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	base := filepath.Join(tempDir, "apps")
	_ = os.MkdirAll(base, 0755)

	// Valid subpath
	safe, err := SafePath(base, "app-123")
	if err != nil || safe != filepath.Join(base, "app-123") {
		t.Errorf("Expected valid safe path, got %s / %v", safe, err)
	}

	// Malicious traversal attempts
	traversals := []string{
		"../../etc/passwd",
		"app-1/../../etc",
		"/etc/shadow",
		"../apps",
	}

	for _, trav := range traversals {
		_, err := SafePath(base, trav)
		if err == nil {
			t.Errorf("Path traversal attack %q was NOT rejected!", trav)
		}
	}
}

func TestAppUnixUsername(t *testing.T) {
	user := AppUnixUsername("runpilot-app", "web-service-10")
	if user != "runpilot-app-web-service-10" {
		t.Errorf("Unexpected derived username: %s", user)
	}

	// Test 32-char truncation
	longUser := AppUnixUsername("runpilot-app", "very-very-long-application-identifier-99999")
	if len(longUser) > 31 {
		t.Errorf("Username exceeded 31 chars: %s (len %d)", longUser, len(longUser))
	}
}
