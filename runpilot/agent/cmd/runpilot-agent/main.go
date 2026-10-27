package main

import (
	"flag"
	"log"

	"runpilot/agent/internal/agentapi"
	"runpilot/agent/internal/config"
)

func main() {
	configPath := flag.String("config", "/etc/runpilot/agent.json", "Path to agent configuration file")
	flag.Parse()

	log.Printf("[RunPilot Agent] Initializing Host Daemon v1.0.0...")
	cfg, err := config.LoadAgentConfig(*configPath)
	if err != nil {
		log.Fatalf("[RunPilot Agent] Failed to load configuration: %v", err)
	}

	server := agentapi.NewServer(cfg)
	log.Printf("[RunPilot Agent] Listening on http://%s:%d (systemd supervisor, cgroups v2)",
		cfg.Agent.Host, cfg.Agent.Port)

	if err := server.Start(); err != nil {
		log.Fatalf("[RunPilot Agent] Server exited with error: %v", err)
	}
}
