package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"runpilot/control-plane/internal/api"
	"runpilot/control-plane/internal/audit"
	"runpilot/control-plane/internal/auth"
	"runpilot/control-plane/internal/config"
	"runpilot/control-plane/internal/db"
	"runpilot/control-plane/internal/proxy"
	"runpilot/control-plane/internal/services"
)

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	migrationPath := flag.String("migrations", "", "Path to migration SQL file")
	port := flag.Int("port", 0, "Override HTTP server port")
	resetUser := flag.String("reset-user", "", "Username or email to update/reset credentials via CLI")
	newPass := flag.String("new-pass", "", "New password to set for the user")
	newEmail := flag.String("new-email", "", "New email address to set for the user")
	newUsername := flag.String("new-username", "", "New username to set for the user")
	flag.Parse()

	log.Println("[RunPilot Server] Initializing Control Plane v1.0.0...")

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("[RunPilot Server] Failed to load configuration: %v", err)
	}

	if *port > 0 {
		cfg.Server.Port = *port
	}

	// Locate migration SQL
	migFile := *migrationPath
	if migFile == "" {
		for _, candidate := range []string{
			"/opt/runpilot-repo/runpilot/control-plane/migrations/001_initial_schema.sql",
			"/opt/runpilot/migrations/001_initial_schema.sql",
			"/opt/runpilot/migrations/migrations/001_initial_schema.sql",
			"control-plane/migrations/001_initial_schema.sql",
			"/tmp/runpilot-repo/runpilot/control-plane/migrations/001_initial_schema.sql",
			filepath.Join(os.Getenv("RUNPILOT_DIR"), "migrations", "001_initial_schema.sql"),
		} {
			if _, err := os.Stat(candidate); err == nil {
				migFile = candidate
				break
			}
		}
	}

	log.Printf("[RunPilot Server] Connecting to database (%s) and running migrations from %s...",
		cfg.Database.Driver, migFile)

	database, err := db.ConnectAndMigrate(cfg, migFile)
	if err != nil {
		log.Fatalf("[RunPilot Server] Database initialization failed: %v", err)
	}

	agentClient := services.NewAgentClient(cfg)
	authService := auth.NewAuthService(database, cfg)
	appService := services.NewAppService(database, cfg, agentClient)
	pkgService := services.NewPackageService(database)
	auditLogger := audit.NewAuditLogger(database)

	// Handle direct CLI credential reset if requested
	if *resetUser != "" {
		if *newPass != "" {
			if err := authService.ResetPasswordDirectly(*resetUser, *newPass); err != nil {
				log.Fatalf("[CLI Error] Failed to reset password: %v", err)
			}
			log.Printf("[CLI Success] Successfully updated password for user '%s'!", *resetUser)
		}
		if *newEmail != "" {
			_, err := database.Exec(`UPDATE users SET email = ?, updated_at = CURRENT_TIMESTAMP WHERE lower(email) = ? OR lower(COALESCE(username, '')) = ?`, strings.ToLower(*newEmail), strings.ToLower(*resetUser), strings.ToLower(*resetUser))
			if err != nil {
				log.Fatalf("[CLI Error] Failed to update email: %v", err)
			}
			log.Printf("[CLI Success] Successfully updated email to '%s' for user '%s'!", *newEmail, *resetUser)
		}
		if *newUsername != "" {
			_, err := database.Exec(`UPDATE users SET username = ?, updated_at = CURRENT_TIMESTAMP WHERE lower(email) = ? OR lower(COALESCE(username, '')) = ?`, strings.ToLower(*newUsername), strings.ToLower(*resetUser), strings.ToLower(*resetUser))
			if err != nil {
				log.Fatalf("[CLI Error] Failed to update username: %v", err)
			}
			log.Printf("[CLI Success] Successfully updated username to '%s' for user '%s'!", *newUsername, *resetUser)
		}
		os.Exit(0)
	}

	apiHandler := api.NewAPI(cfg, authService, appService, pkgService, agentClient, auditLogger)
	nativeRouter := proxy.NewRouter(apiHandler.Handler(), appService, cfg.Server.Host)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("[RunPilot Server] Native Standalone Engine listening on http://%s", addr)
	log.Printf("[RunPilot Server] Architecture: 100%% Standalone Pure-Go Engine (Zero external server packages, native reverse proxy)")
	log.Printf("[RunPilot Server] Runtime Ecosystem: Node.js & Python (Zero PHP/Composer requirement)")

	// If running on standard HTTP port 80, also open companion admin port 2080 so both http://IP and http://IP:2080 work
	if cfg.Server.Port == 80 {
		go func() {
			adminAddr := fmt.Sprintf("%s:2080", cfg.Server.Host)
			log.Printf("[RunPilot Server] Companion Admin Panel listening on http://%s", adminAddr)
			if err := http.ListenAndServe(adminAddr, nativeRouter); err != nil {
				log.Printf("[RunPilot Server] Port 2080 listener note: %v", err)
			}
		}()
	}

	if err := http.ListenAndServe(addr, nativeRouter); err != nil {
		log.Fatalf("[RunPilot Server] Server exited with error: %v", err)
	}
}
