package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"runpilot/shared/contracts"
	"runpilot/shared/models"
)

type Config struct {
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
	Email    string `json:"email"`
}

type CLIClient struct {
	config Config
	http   *http.Client
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".runpilot", "credentials.json")
}

func LoadConfig() (*Config, error) {
	p := configPath()
	data, err := os.ReadFile(p)
	if err != nil {
		return &Config{Endpoint: "http://127.0.0.1:8080"}, nil
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{Endpoint: "http://127.0.0.1:8080"}, nil
	}
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	p := configPath()
	_ = os.MkdirAll(filepath.Dir(p), 0700)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

func NewCLIClient() (*CLIClient, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return &CLIClient{
		config: *cfg,
		http:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *CLIClient) request(method, path string, body interface{}, target interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	url := fmt.Sprintf("%s%s", c.config.Endpoint, path)
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return err
	}

	if c.config.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.Token))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned error (status %d): %s", resp.StatusCode, string(respBytes))
	}

	if target != nil {
		return json.Unmarshal(respBytes, target)
	}
	return nil
}

func (c *CLIClient) Login(email, password, endpoint string) error {
	if endpoint != "" {
		c.config.Endpoint = endpoint
	}
	payload := map[string]string{
		"email":    email,
		"password": password,
	}

	var resp struct {
		Token string      `json:"token"`
		User  models.User `json:"user"`
	}

	if err := c.request(http.MethodPost, "/api/v1/auth/login", payload, &resp); err != nil {
		return err
	}

	c.config.Token = resp.Token
	c.config.Email = resp.User.Email
	return SaveConfig(&c.config)
}

func (c *CLIClient) ListApps() ([]models.Application, error) {
	var apps []models.Application
	err := c.request(http.MethodGet, "/api/v1/applications", nil, &apps)
	return apps, err
}

func (c *CLIClient) CreateApp(app models.Application) (*models.Application, error) {
	var created models.Application
	err := c.request(http.MethodPost, "/api/v1/applications", app, &created)
	return &created, err
}

func (c *CLIClient) Deploy(appID string) (*models.Deployment, error) {
	var dep models.Deployment
	err := c.request(http.MethodPost, fmt.Sprintf("/api/v1/applications/%s/deploy", appID), nil, &dep)
	return &dep, err
}

func (c *CLIClient) Restart(appID string) (*contracts.AgentProcessStatusResponse, error) {
	var stat contracts.AgentProcessStatusResponse
	err := c.request(http.MethodPost, fmt.Sprintf("/api/v1/applications/%s/restart", appID), nil, &stat)
	return &stat, err
}

func (c *CLIClient) Status(appID string) (*models.Application, error) {
	var app models.Application
	err := c.request(http.MethodGet, fmt.Sprintf("/api/v1/applications/%s", appID), nil, &app)
	return &app, err
}

func (c *CLIClient) Logs(appID string, lines int) ([]string, error) {
	var resp struct {
		Lines []string `json:"lines"`
	}
	err := c.request(http.MethodGet, fmt.Sprintf("/api/v1/applications/%s/logs?lines=%d", appID, lines), nil, &resp)
	return resp.Lines, err
}

func (c *CLIClient) ListServers() ([]map[string]interface{}, error) {
	var servers []map[string]interface{}
	err := c.request(http.MethodGet, "/api/v1/servers", nil, &servers)
	return servers, err
}
