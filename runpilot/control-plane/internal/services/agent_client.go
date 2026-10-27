package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"runpilot/control-plane/internal/config"
	"runpilot/shared/contracts"
	"runpilot/shared/crypto"
)

type AgentClient struct {
	endpoint  string
	secretKey []byte
	client    *http.Client
}

func NewAgentClient(cfg *config.Config) *AgentClient {
	endpoint := cfg.Agent.Endpoint
	if endpoint == "" {
		endpoint = "http://127.0.0.1:9090"
	}
	sec := []byte(cfg.Agent.SecretKey)
	if len(sec) == 0 {
		sec = []byte("agent-shared-secret-key-runpilot")
	}

	return &AgentClient{
		endpoint:  endpoint,
		secretKey: sec,
		client:    &http.Client{Timeout: 15 * time.Minute},
	}
}

func (c *AgentClient) doRequest(method, path string, payload interface{}, target interface{}) error {
	var bodyBytes []byte
	var err error

	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}

	url := fmt.Sprintf("%s%s", c.endpoint, path)
	req, err := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	// Sign request with HMAC
	ts, sig := crypto.SignAgentRequest(c.secretKey, bodyBytes)
	req.Header.Set("X-RunPilot-Timestamp", ts)
	req.Header.Set("X-RunPilot-Signature", sig)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("agent communication error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read agent response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("agent error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if target != nil {
		if err := json.Unmarshal(respBody, target); err != nil {
			return fmt.Errorf("failed to decode agent response: %w", err)
		}
	}

	return nil
}

func (c *AgentClient) Deploy(req contracts.AgentDeployRequest) (*contracts.AgentDeployResponse, error) {
	var resp contracts.AgentDeployResponse
	err := c.doRequest(http.MethodPost, "/api/v1/agent/deploy", req, &resp)
	return &resp, err
}

func (c *AgentClient) ProcessAction(appID, action string) (*contracts.AgentProcessStatusResponse, error) {
	req := contracts.AgentProcessActionRequest{
		AppID:  appID,
		Action: action,
	}
	var resp contracts.AgentProcessStatusResponse
	err := c.doRequest(http.MethodPost, "/api/v1/agent/process", req, &resp)
	return &resp, err
}

func (c *AgentClient) GetStatus(appID string) (*contracts.AgentProcessStatusResponse, error) {
	path := fmt.Sprintf("/api/v1/agent/status?app_id=%s", appID)
	var resp contracts.AgentProcessStatusResponse
	err := c.doRequest(http.MethodGet, path, nil, &resp)
	return &resp, err
}

func (c *AgentClient) GetLogs(appID string, lines int) ([]string, error) {
	path := fmt.Sprintf("/api/v1/agent/logs?app_id=%s&lines=%d", appID, lines)
	var resp contracts.AgentLogsResponse
	err := c.doRequest(http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Lines, nil
}

func (c *AgentClient) Rollback(appID, releaseID string) error {
	payload := map[string]string{
		"app_id":     appID,
		"release_id": releaseID,
	}
	return c.doRequest(http.MethodPost, "/api/v1/agent/rollback", payload, nil)
}

func (c *AgentClient) ConfigureNginx(req contracts.AgentNginxConfigRequest) error {
	return c.doRequest(http.MethodPost, "/api/v1/agent/nginx", req, nil)
}

func (c *AgentClient) GetServerMetrics() (*contracts.AgentServerMetrics, error) {
	var metrics contracts.AgentServerMetrics
	err := c.doRequest(http.MethodGet, "/api/v1/agent/metrics", nil, &metrics)
	return &metrics, err
}
