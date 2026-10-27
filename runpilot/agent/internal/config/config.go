package config

import (
	"encoding/json"
	"os"
)

type AgentConfig struct {
	Agent AgentSettings `json:"agent"`
}

type AgentSettings struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	SecretFile         string `json:"secret_file"`
	SecretKey          string `json:"secret_key,omitempty"`
	DataDir            string `json:"data_dir"`
	AppsDir            string `json:"apps_dir"`
	RuntimesDir        string `json:"runtimes_dir"`
	LogDir             string `json:"log_dir"`
	NginxSitesDir      string `json:"nginx_sites_dir"`
	SystemdUnitPrefix  string `json:"systemd_unit_prefix"`
	SystemdServicePath string `json:"systemd_service_path"`
}

func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		Agent: AgentSettings{
			Host:               "127.0.0.1",
			Port:               9090,
			SecretFile:         "",
			SecretKey:          "agent-shared-secret-key-runpilot",
			DataDir:            "/srv/runpilot",
			AppsDir:            "/srv/runpilot/apps",
			RuntimesDir:        "/srv/runpilot/runtimes",
			LogDir:             "/var/log/runpilot",
			NginxSitesDir:      "/etc/nginx/conf.d",
			SystemdUnitPrefix:  "runpilot-app",
			SystemdServicePath: "/etc/systemd/system",
		},
	}
}

func LoadAgentConfig(path string) (*AgentConfig, error) {
	cfg := DefaultAgentConfig()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if cfg.Agent.SecretFile != "" {
		sec, err := os.ReadFile(cfg.Agent.SecretFile)
		if err == nil {
			cfg.Agent.SecretKey = string(sec)
		}
	}

	return cfg, nil
}
