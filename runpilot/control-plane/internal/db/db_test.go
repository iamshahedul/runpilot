package db

import (
	"os"
	"path/filepath"
	"testing"

	"runpilot/control-plane/internal/config"
	"runpilot/shared/crypto"
)

func TestDatabaseMigrationAndAdminSeed(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runpilot-db-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	cfg := config.DefaultConfig()
	cfg.Database.Driver = "sqlite"
	cfg.Database.SQLitePath = dbPath

	migrationPath := filepath.Join("..", "..", "migrations", "001_initial_schema.sql")

	database, err := ConnectAndMigrate(cfg, migrationPath)
	if err != nil {
		t.Fatalf("ConnectAndMigrate failed: %v", err)
	}
	defer database.Close()

	// Verify admin exists
	var email, role, hash string
	err = database.QueryRow("SELECT email, role, password_hash FROM users WHERE email = 'admin@runpilot.local'").Scan(&email, &role, &hash)
	if err != nil {
		t.Fatalf("Failed to query default admin user: %v", err)
	}

	if email != "admin@runpilot.local" || role != "admin" {
		t.Errorf("Unexpected admin user: %s / %s", email, role)
	}

	if !crypto.CheckPasswordHash("RunPilot2026!", hash) {
		t.Errorf("Admin password verification failed")
	}

	// Verify server seeded
	var serverName string
	err = database.QueryRow("SELECT name FROM servers WHERE id = 1").Scan(&serverName)
	if err != nil {
		t.Fatalf("Failed to query seeded server: %v", err)
	}
	if serverName != "Primary Node" {
		t.Errorf("Unexpected server name: %s", serverName)
	}
}
