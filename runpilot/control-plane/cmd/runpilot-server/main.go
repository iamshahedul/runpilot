package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"runpilot/control-plane/internal/api"
	"runpilot/control-plane/internal/audit"
	"runpilot/control-plane/internal/auth"
	"runpilot/control-plane/internal/config"
	"runpilot/control-plane/internal/db"
	"runpilot/control-plane/internal/services"
)

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	migrationPath := flag.String("migrations", "", "Path to migration SQL file")
	port := flag.Int("port", 0, "Override HTTP server port")
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
			"control-plane/migrations/001_initial_schema.sql",
			"/opt/runpilot/migrations/001_initial_schema.sql",
			"/opt/runpilot/migrations/migrations/001_initial_schema.sql",
			"/tmp/runpilot-repo/runpilot/control-plane/migrations/001_initial_schema.sql",
			filepath.Join(os.Getenv("RUNPILOT_DIR"), "migrations", "001_initial_schema.sql"),
		} {
			if _, err := os.Stat(candidate); err == nil {
				migFile = candidate
				break
			}
		}
	}

	if migFile == "" {
		migFile = "control-plane/migrations/001_initial_schema.sql"
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
	auditLogger := audit.NewAuditLogger(database)

	apiHandler := api.NewAPI(cfg, authService, appService, agentClient, auditLogger)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("[RunPilot Server] Web Dashboard and REST API listening on http://%s", addr)
	log.Printf("[RunPilot Server] Architecture: Native processes via systemd, cgroups v2, Nginx")
	log.Printf("[RunPilot Server] Runtime Ecosystem: Node.js & Python (Zero PHP/Composer requirement)")

	if err := http.ListenAndServe(addr, apiHandler.Handler()); err != nil {
		log.Fatalf("[RunPilot Server] Server exited with error: %v", err)
	}
}
