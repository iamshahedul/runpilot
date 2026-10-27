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
	// Auto-migrate: Ensure username column exists on users table
	_, _ = d.Exec(`ALTER TABLE users ADD COLUMN username VARCHAR(100)`)
	_, _ = d.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username)`)

	// Auto-migrate: Ensure packages table exists
	_, _ = d.Exec(`
		CREATE TABLE IF NOT EXISTS packages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name VARCHAR(100) NOT NULL UNIQUE,
			slug VARCHAR(100) NOT NULL UNIQUE,
			memory_max VARCHAR(20) NOT NULL DEFAULT '512M',
			cpu_quota VARCHAR(20) NOT NULL DEFAULT '100%',
			tasks_max INTEGER NOT NULL DEFAULT 128,
			disk_quota VARCHAR(20) NOT NULL DEFAULT '5G',
			max_apps INTEGER NOT NULL DEFAULT 1,
			max_domains INTEGER NOT NULL DEFAULT 2,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)

	// Seed default cPanel-style packages
	_, _ = d.Exec(`
		INSERT OR IGNORE INTO packages (id, name, slug, memory_max, cpu_quota, tasks_max, disk_quota, max_apps, max_domains) VALUES
		(1, 'Starter', 'starter', '512M', '100%', 128, '5G', 1, 2),
		(2, 'Professional', 'professional', '1G', '150%', 256, '15G', 3, 5),
		(3, 'Business', 'business', '2G', '200%', 512, '30G', 10, 15)
	`)

	var count int
	err := d.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}

	adminEmail := os.Getenv("RUNPILOT_ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@runpilot.local"
	}
	adminPass := os.Getenv("RUNPILOT_ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "RunPilot2026!"
	}

	if count == 0 {
		hash, err := crypto.HashPassword(adminPass)
		if err != nil {
			return err
		}
		_, err = d.Exec(`
			INSERT INTO users (username, email, password_hash, full_name, role, is_active)
			VALUES ('admin', ?, ?, 'Platform Administrator', 'admin', 1)
		`, adminEmail, hash)
		if err != nil {
			return err
		}
	} else {
		// Ensure first user has username 'admin' if empty
		_, _ = d.Exec(`UPDATE users SET username = 'admin' WHERE id = 1 AND (username IS NULL OR username = '')`)
	}
	return nil
}
