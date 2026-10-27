package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"

	"runpilot/control-plane/internal/config"
	"runpilot/shared/crypto"
)

type DB struct {
	*sql.DB
	Driver string
}

func ConnectAndMigrate(cfg *config.Config, migrationSQLPath string) (*DB, error) {
	var connStr string
	driver := cfg.Database.Driver

	if driver == "mysql" {
		connStr = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Name,
		)
	} else {
		driver = "sqlite"
		connStr = cfg.Database.SQLitePath
		if connStr == "" {
			connStr = "runpilot.db"
		}
	}

	db, err := sql.Open(driver, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	instance := &DB{DB: db, Driver: driver}

	// Run migration
	if err := instance.RunMigration(migrationSQLPath); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	// Ensure default admin user exists
	if err := instance.EnsureDefaultAdmin(); err != nil {
		return nil, fmt.Errorf("failed to seed default admin: %w", err)
	}

	return instance, nil
}

func (d *DB) RunMigration(sqlFilePath string) error {
	content, err := os.ReadFile(sqlFilePath)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", sqlFilePath, err)
	}

	// Strip single line comments first
	var cleanLines []string
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		cleanLines = append(cleanLines, line)
	}
	cleanContent := strings.Join(cleanLines, "\n")

	// Split statements by semicolon
	statements := strings.Split(cleanContent, ";")
	for _, rawStmt := range statements {
		stmt := strings.TrimSpace(rawStmt)
		if stmt == "" {
			continue
		}
		if _, err := d.Exec(stmt); err != nil {
			// Ignore table already exists or duplicate index errors
			if !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate") {
				return fmt.Errorf("error executing stmt [%s]: %w", stmt, err)
			}
		}
	}
	return nil
}

func (d *DB) EnsureDefaultAdmin() error {
	var count int
	err := d.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		hash, err := crypto.HashPassword("RunPilot2026!")
		if err != nil {
			return err
		}
		_, err = d.Exec(`
			INSERT INTO users (email, password_hash, full_name, role, is_active)
			VALUES (?, ?, ?, ?, 1)
		`, "admin@runpilot.local", hash, "Platform Administrator", "admin")
		if err != nil {
			return err
		}
	}
	return nil
}
