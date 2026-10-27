package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"runpilot/control-plane/internal/audit"
	"runpilot/control-plane/internal/auth"
	servercfg "runpilot/control-plane/internal/config"
	"runpilot/control-plane/internal/db"
	"runpilot/control-plane/internal/services"
	"runpilot/shared/contracts"
	"runpilot/shared/crypto"
	"runpilot/shared/models"
)

func TestAPILifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runpilot-api-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	secretKey := "test-agent-secret-key-32bytes!!"

	// 1. Mock Agent HTTP Server
	mockAgent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts := r.Header.Get("X-RunPilot-Timestamp")
		sig := r.Header.Get("X-RunPilot-Signature")
		body := []byte{}
		_ = crypto.VerifyAgentRequest([]byte(secretKey), body, ts, sig)

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/agent/status":
			_ = json.NewEncoder(w).Encode(contracts.AgentProcessStatusResponse{
				AppID: r.URL.Query().Get("app_id"),
				State: contracts.ProcessRunning,
				PID:   1234,
			})
		case "/api/v1/agent/process":
			_ = json.NewEncoder(w).Encode(contracts.AgentProcessStatusResponse{
				State: contracts.ProcessRunning,
				PID:   1234,
			})
		case "/api/v1/agent/deploy":
			_ = json.NewEncoder(w).Encode(contracts.AgentDeployResponse{
				Success:     true,
				ReleaseID:   "rel_test_001",
				BuildOutput: "[RunPilot] Mock Build Success\n",
			})
		case "/api/v1/agent/logs":
			_ = json.NewEncoder(w).Encode(contracts.AgentLogsResponse{
				Lines: []string{"[RunPilot App] Started successfully"},
			})
		case "/api/v1/agent/metrics":
			_ = json.NewEncoder(w).Encode(contracts.AgentServerMetrics{
				Hostname:    "test-node",
				OS:          "linux",
				CPUUsagePct: 10.5,
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		}
	}))
	defer mockAgent.Close()

	// 2. Setup Control Plane
	serverConfig := servercfg.DefaultConfig()
	serverConfig.Database.Driver = "sqlite"
	serverConfig.Database.SQLitePath = filepath.Join(tempDir, "test.db")
	serverConfig.Server.WebRoot = filepath.Join("..", "..", "web")
	serverConfig.Server.JWTSecret = "jwt-secret-key-for-test-32bytes!"
	serverConfig.Server.MasterKey = "master-aes-key-for-test-32bytes!"
	serverConfig.Agent.Endpoint = mockAgent.URL
	serverConfig.Agent.SecretKey = secretKey

	migrationPath := filepath.Join("..", "..", "migrations", "001_initial_schema.sql")
	database, err := db.ConnectAndMigrate(serverConfig, migrationPath)
	if err != nil {
		t.Fatalf("Database initialization failed: %v", err)
	}
	defer database.Close()

	agentClient := services.NewAgentClient(serverConfig)
	authService := auth.NewAuthService(database, serverConfig)
	appService := services.NewAppService(database, serverConfig, agentClient)
	auditLogger := audit.NewAuditLogger(database)

	apiHandler := NewAPI(serverConfig, authService, appService, agentClient, auditLogger)
	serverHttpServer := httptest.NewServer(apiHandler.Handler())
	defer serverHttpServer.Close()

	// 3. Test Authentication
	loginPayload := map[string]string{
		"email":    "admin@runpilot.local",
		"password": "RunPilot2026!",
	}
	loginBody, _ := json.Marshal(loginPayload)
	resp, err := http.Post(serverHttpServer.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Login failed with status %d: %v", resp.StatusCode, err)
	}

	var loginResp struct {
		Token string      `json:"token"`
		User  models.User `json:"user"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&loginResp)
	resp.Body.Close()

	client := &http.Client{}
	doAuthReq := func(method, path string, body interface{}) (*http.Response, error) {
		var bodyReader *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(b)
		} else {
			bodyReader = bytes.NewReader([]byte{})
		}
		req, err := http.NewRequest(method, serverHttpServer.URL+path, bodyReader)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+loginResp.Token)
		req.Header.Set("Content-Type", "application/json")
		return client.Do(req)
	}

	// 4. Create Node.js App
	nodeApp := models.Application{
		ID:             "express-api",
		Name:           "Express Production API",
		RuntimeType:    contracts.RuntimeNodeJS,
		RuntimeVersion: "20.x",
		GitRepo:        "https://github.com/example/express-api.git",
		GitBranch:      "main",
		Port:           3000,
		BuildCommand:   "npm install",
		StartCommand:   "node server.js",
		HealthPath:     "/health",
	}

	res, err := doAuthReq(http.MethodPost, "/api/v1/applications", nodeApp)
	if err != nil || res.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create app: %v", err)
	}
	res.Body.Close()

	// 5. Create Python App
	pyApp := models.Application{
		ID:             "fastapi-service",
		Name:           "FastAPI Microservice",
		RuntimeType:    contracts.RuntimePython,
		RuntimeVersion: "3.11",
		GitRepo:        "https://github.com/example/fastapi.git",
		GitBranch:      "main",
		Port:           8000,
		BuildCommand:   "pip install -r requirements.txt",
		StartCommand:   "uvicorn main:app",
	}

	res, err = doAuthReq(http.MethodPost, "/api/v1/applications", pyApp)
	if err != nil || res.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create Python app: %v", err)
	}
	res.Body.Close()

	// 6. List Apps
	res, err = doAuthReq(http.MethodGet, "/api/v1/applications", nil)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("Failed to list apps: %v", err)
	}
	var apps []models.Application
	_ = json.NewDecoder(res.Body).Decode(&apps)
	res.Body.Close()

	if len(apps) != 2 {
		t.Errorf("Expected 2 apps, got %d", len(apps))
	}

	// 7. Deploy App
	res, err = doAuthReq(http.MethodPost, "/api/v1/applications/express-api/deploy", nil)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("Failed to deploy app: %v", err)
	}
	var dep models.Deployment
	_ = json.NewDecoder(res.Body).Decode(&dep)
	res.Body.Close()

	if dep.State != contracts.DeployStateSuccessful {
		t.Errorf("Expected deployment state successful, got %s", dep.State)
	}

	// 8. Attach Domain
	res, err = doAuthReq(http.MethodPost, "/api/v1/applications/express-api/domains", map[string]string{"domain": "api.example.com"})
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("Failed to attach domain: %v", err)
	}
	res.Body.Close()

	// 9. Process Control (Restart)
	res, err = doAuthReq(http.MethodPost, "/api/v1/applications/express-api/restart", nil)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("Failed to restart app: %v", err)
	}
	res.Body.Close()

	// 10. Audit Events check
	res, err = doAuthReq(http.MethodGet, "/api/v1/audit-events", nil)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get audit events: %v", err)
	}
	var events []models.AuditEvent
	_ = json.NewDecoder(res.Body).Decode(&events)
	res.Body.Close()

	if len(events) < 4 {
		t.Errorf("Expected at least 4 audit events, got %d", len(events))
	}
}
