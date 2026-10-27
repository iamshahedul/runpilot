package agentapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"runpilot/agent/internal/config"
	"runpilot/agent/internal/nginx"
	"runpilot/agent/internal/runner"
	"runpilot/agent/internal/runtimes/nodejs"
	"runpilot/agent/internal/runtimes/python"
	"runpilot/agent/internal/security"
	"runpilot/agent/internal/systemd"
	"runpilot/shared/contracts"
	"runpilot/shared/crypto"
)

type Server struct {
	cfg       *config.AgentConfig
	workspace *runner.Workspace
	systemd   *systemd.Manager
	nodeMgr   *nodejs.NodeRuntimeManager
	pyMgr     *python.PythonRuntimeManager
	nginxMgr  *nginx.NginxManager
	mux       *http.ServeMux
}

func NewServer(cfg *config.AgentConfig) *Server {
	ws := runner.NewWorkspace(cfg.Agent.AppsDir)
	sys := systemd.NewManager(cfg.Agent.SystemdUnitPrefix, cfg.Agent.SystemdServicePath)
	node := nodejs.NewNodeRuntimeManager(cfg.Agent.RuntimesDir)
	py := python.NewPythonRuntimeManager(cfg.Agent.RuntimesDir)
	ng := nginx.NewNginxManager(cfg.Agent.NginxSitesDir)

	s := &Server{
		cfg:       cfg,
		workspace: ws,
		systemd:   sys,
		nodeMgr:   node,
		pyMgr:     py,
		nginxMgr:  ng,
		mux:       http.NewServeMux(),
	}

	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/v1/agent/health", s.handleHealth)
	s.mux.HandleFunc("/api/v1/agent/deploy", s.authMiddleware(s.handleDeploy))
	s.mux.HandleFunc("/api/v1/agent/process", s.authMiddleware(s.handleProcessAction))
	s.mux.HandleFunc("/api/v1/agent/status", s.authMiddleware(s.handleStatus))
	s.mux.HandleFunc("/api/v1/agent/logs", s.authMiddleware(s.handleLogs))
	s.mux.HandleFunc("/api/v1/agent/rollback", s.authMiddleware(s.handleRollback))
	s.mux.HandleFunc("/api/v1/agent/nginx", s.authMiddleware(s.handleNginxConfig))
	s.mux.HandleFunc("/api/v1/agent/metrics", s.authMiddleware(s.handleMetrics))
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secretKey := []byte(s.cfg.Agent.SecretKey)
		if len(secretKey) == 0 {
			secretKey = []byte("agent-shared-secret-key-runpilot")
		}

		ts := r.Header.Get("X-RunPilot-Timestamp")
		sig := r.Header.Get("X-RunPilot-Signature")

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
			return
		}
		// Reset body reader
		r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

		if err := crypto.VerifyAgentRequest(secretKey, bodyBytes, ts, sig); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"unauthorized: %s"}`, err.Error()), http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": "1.0.0",
		"time":    time.Now().UTC(),
	})
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req contracts.AgentDeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid json: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	if err := security.ValidateAppID(req.AppID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	unixUser := security.AppUnixUsername(s.cfg.Agent.SystemdUnitPrefix, req.AppID)
	_ = security.EnsureAppUser(unixUser)

	// 1. Setup workspace directories
	if err := s.workspace.SetupAppDirectories(req.AppID); err != nil {
		s.respondDeployError(w, req.ReleaseID, "", err)
		return
	}

	// 2. Write environment variables (.env)
	if err := s.workspace.WriteEnvironmentFile(req.AppID, req.EnvVars); err != nil {
		s.respondDeployError(w, req.ReleaseID, "", err)
		return
	}

	// 3. Prepare release folder
	releaseDir, err := s.workspace.PrepareReleaseDirectory(req.AppID, req.ReleaseID)
	if err != nil {
		s.respondDeployError(w, req.ReleaseID, "", err)
		return
	}

	// 4. Git clone or checkout into release directory
	var buildLog strings.Builder
	if req.GitRepo != "" {
		buildLog.WriteString(fmt.Sprintf("[RunPilot] Fetching repository: %s (branch: %s)...\n", req.GitRepo, req.GitBranch))
		branch := req.GitBranch
		if branch == "" {
			branch = "main"
		}

		cloneCmd := exec.Command("git", "clone", "--depth", "1", "--branch", branch, req.GitRepo, releaseDir)
		cloneOut, err := cloneCmd.CombinedOutput()
		buildLog.Write(cloneOut)
		if err != nil {
			// Try without branch if specific branch clone failed
			cloneCmdFallback := exec.Command("git", "clone", "--depth", "1", req.GitRepo, releaseDir)
			fbOut, fbErr := cloneCmdFallback.CombinedOutput()
			buildLog.Write(fbOut)
			if fbErr != nil {
				s.respondDeployError(w, req.ReleaseID, buildLog.String(), fmt.Errorf("git checkout failed: %w", fbErr))
				return
			}
		}
	}

	// 5. Execute runtime-specific build
	ctx := context.Background()
	var startCommand string

	switch req.RuntimeType {
	case contracts.RuntimeNodeJS:
		buildLog.WriteString("[RunPilot] Building Node.js application...\n")
		out, err := s.nodeMgr.BuildRelease(ctx, releaseDir, req.BuildCmd, req.EnvVars)
		buildLog.WriteString(out)
		if err != nil {
			s.respondDeployError(w, req.ReleaseID, buildLog.String(), err)
			return
		}
		startCommand = s.nodeMgr.ResolveStartCommand(releaseDir, req.StartCmd)

	case contracts.RuntimePython:
		buildLog.WriteString("[RunPilot] Building Python application and virtual environment...\n")
		out, err := s.pyMgr.BuildRelease(ctx, releaseDir, req.BuildCmd, req.EnvVars)
		buildLog.WriteString(out)
		if err != nil {
			s.respondDeployError(w, req.ReleaseID, buildLog.String(), err)
			return
		}
		startCommand = s.pyMgr.ResolveStartCommand(releaseDir, req.StartCmd, req.Port)

	default:
		s.respondDeployError(w, req.ReleaseID, buildLog.String(), fmt.Errorf("unsupported runtime type: %s", req.RuntimeType))
		return
	}

	// 6. Formulate systemd unit and sandbox
	appRoot, _ := s.workspace.AppDir(req.AppID)
	currentDir := filepath.Join(appRoot, "current")
	envFile := filepath.Join(appRoot, "shared", ".env")

	if err := s.systemd.WriteUnit(req.AppID, unixUser, currentDir, envFile, startCommand, req.ResourceLimits); err != nil {
		s.respondDeployError(w, req.ReleaseID, buildLog.String(), err)
		return
	}

	// 7. Atomically activate release symlink
	if err := s.workspace.ActivateRelease(req.AppID, req.ReleaseID); err != nil {
		s.respondDeployError(w, req.ReleaseID, buildLog.String(), err)
		return
	}

	// 8. Restart / Start service
	if err := s.systemd.Restart(req.AppID, currentDir, startCommand, req.EnvVars); err != nil {
		s.respondDeployError(w, req.ReleaseID, buildLog.String(), err)
		return
	}

	buildLog.WriteString("\n[RunPilot] Application successfully deployed and started!\n")

	resp := contracts.AgentDeployResponse{
		Success:     true,
		ReleaseID:   req.ReleaseID,
		BuildOutput: buildLog.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) respondDeployError(w http.ResponseWriter, releaseID string, logOutput string, err error) {
	resp := contracts.AgentDeployResponse{
		Success:      false,
		ReleaseID:    releaseID,
		BuildOutput:  logOutput,
		ErrorMessage: err.Error(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleProcessAction(w http.ResponseWriter, r *http.Request) {
	var req contracts.AgentProcessActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	appRoot, err := s.workspace.AppDir(req.AppID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	currentDir := filepath.Join(appRoot, "current")

	switch req.Action {
	case "start":
		_ = s.systemd.Start(req.AppID, currentDir, "node server.js", nil)
	case "stop":
		_ = s.systemd.Stop(req.AppID)
	case "restart":
		_ = s.systemd.Restart(req.AppID, currentDir, "node server.js", nil)
	}

	status := s.systemd.Status(req.AppID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		http.Error(w, "missing app_id parameter", http.StatusBadRequest)
		return
	}

	status := s.systemd.Status(appID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("app_id")
	linesStr := r.URL.Query().Get("lines")
	lines, _ := strconv.Atoi(linesStr)
	if lines <= 0 {
		lines = 50
	}

	logLines := s.systemd.Logs(appID, lines)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(contracts.AgentLogsResponse{
		AppID: appID,
		Lines: logLines,
	})
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AppID     string `json:"app_id"`
		ReleaseID string `json:"release_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.workspace.RollbackRelease(req.AppID, req.ReleaseID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	appRoot, _ := s.workspace.AppDir(req.AppID)
	currentDir := filepath.Join(appRoot, "current")
	_ = s.systemd.Restart(req.AppID, currentDir, "node server.js", nil)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Rolled back to release %s", req.ReleaseID),
	})
}

func (s *Server) handleNginxConfig(w http.ResponseWriter, r *http.Request) {
	var req contracts.AgentNginxConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.nginxMgr.WriteAndValidate(req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Nginx configuration generated and reloaded successfully",
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	metrics := contracts.AgentServerMetrics{
		Hostname:      hostname,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		CPUCount:      runtime.NumCPU(),
		CPUUsagePct:   12.5,
		MemoryTotalMB: 8192,
		MemoryUsedMB:  2048,
		MemoryFreeMB:  6144,
		DiskTotalGB:   100,
		DiskUsedGB:    25,
		DiskFreeGB:    75,
		ActiveApps:    1,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metrics)
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Agent.Host, s.cfg.Agent.Port)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) Handler() http.Handler {
	return s.mux
}
