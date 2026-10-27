package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Agent    AgentConfig    `json:"agent"`
}

type ServerConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	WebRoot    string `json:"web_root"`
	JWTSecret  string `json:"jwt_secret"`
	MasterKey  string `json:"master_key"`
	DataDir    string `json:"data_dir"`
}

type DatabaseConfig struct {
	Driver     string `json:"driver"` // "mysql" or "sqlite"
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Name       string `json:"name"`
	User       string `json:"user"`
	Password   string `json:"password"`
	SQLitePath string `json:"sqlite_path"`
}

type AgentConfig struct {
	Endpoint   string `json:"endpoint"`
	SecretFile string `json:"secret_file"`
	SecretKey  string `json:"secret_key,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:      "0.0.0.0",
			Port:      8080,
			WebRoot:   "control-plane/web",
			JWTSecret: "runpilot-default-jwt-secret-key-32bytes!",
			MasterKey: "runpilot-default-master-key-32bytes!",
			DataDir:   "/srv/runpilot",
		},
		Database: DatabaseConfig{
			Driver:     "sqlite",
			Host:       "127.0.0.1",
			Port:       3306,
			Name:       "runpilot",
			User:       "runpilot",
			Password:   "",
			SQLitePath: "runpilot.db",
		},
		Agent: AgentConfig{
			Endpoint:   "http://127.0.0.1:9090",
			SecretFile: "",
			SecretKey:  "agent-shared-secret-key-runpilot",
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
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

	// Read agent secret from secret_file if configured
	if cfg.Agent.SecretFile != "" {
		sec, err := os.ReadFile(cfg.Agent.SecretFile)
		if err == nil {
			cfg.Agent.SecretKey = string(sec)
		}
	}

	return cfg, nil
}
