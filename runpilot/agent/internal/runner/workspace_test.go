package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceAtomicReleaseAndRollback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runpilot-workspace-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ws := NewWorkspace(tempDir)
	appID := "test-express-app"

	// 1. Setup directories
	if err := ws.SetupAppDirectories(appID); err != nil {
		t.Fatalf("SetupAppDirectories failed: %v", err)
	}

	// 2. Prepare release 1
	rel1, err := ws.PrepareReleaseDirectory(appID, "rel_001")
	if err != nil {
		t.Fatalf("PrepareReleaseDirectory rel_001 failed: %v", err)
	}
	_ = os.WriteFile(filepath.Join(rel1, "server.js"), []byte("// Release 1"), 0644)

	// Activate release 1
	if err := ws.ActivateRelease(appID, "rel_001"); err != nil {
		t.Fatalf("ActivateRelease rel_001 failed: %v", err)
	}

	appRoot, _ := ws.AppDir(appID)
	curPath := filepath.Join(appRoot, "current")

	// Verify symlink points to rel_001
	target, err := os.Readlink(curPath)
	if err != nil || target != "releases/rel_001" {
		t.Errorf("Current symlink target mismatch: got %q, want %q", target, "releases/rel_001")
	}

	// 3. Prepare release 2
	rel2, err := ws.PrepareReleaseDirectory(appID, "rel_002")
	if err != nil {
		t.Fatalf("PrepareReleaseDirectory rel_002 failed: %v", err)
	}
	_ = os.WriteFile(filepath.Join(rel2, "server.js"), []byte("// Release 2"), 0644)

	// Activate release 2
	if err := ws.ActivateRelease(appID, "rel_002"); err != nil {
		t.Fatalf("ActivateRelease rel_002 failed: %v", err)
	}

	target, _ = os.Readlink(curPath)
	if target != "releases/rel_002" {
		t.Errorf("Current symlink target mismatch after update: got %q, want %q", target, "releases/rel_002")
	}

	// 4. Test Rollback to release 1
	if err := ws.RollbackRelease(appID, "rel_001"); err != nil {
		t.Fatalf("RollbackRelease to rel_001 failed: %v", err)
	}

	target, _ = os.Readlink(curPath)
	if target != "releases/rel_001" {
		t.Errorf("Current symlink target mismatch after rollback: got %q, want %q", target, "releases/rel_001")
	}
}

func TestWorkspaceEnvironmentFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runpilot-env-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ws := NewWorkspace(tempDir)
	appID := "test-env-app"

	_ = ws.SetupAppDirectories(appID)

	vars := map[string]string{
		"PORT":     "3000",
		"NODE_ENV": "production",
		"API_KEY":  "secret-12345",
	}

	if err := ws.WriteEnvironmentFile(appID, vars); err != nil {
		t.Fatalf("WriteEnvironmentFile failed: %v", err)
	}

	appRoot, _ := ws.AppDir(appID)
	envFile := filepath.Join(appRoot, "shared", ".env")
	content, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("Failed to read .env file: %v", err)
	}

	envStr := string(content)
	if !contains(envStr, "PORT=\"3000\"") || !contains(envStr, "API_KEY=\"secret-12345\"") {
		t.Errorf("Missing expected env content: %s", envStr)
	}
}

func contains(s, substr string) bool {
	return filepath.Clean(s) != "" && (s == substr || len(s) >= len(substr) && searchSubstr(s, substr))
}

func searchSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
