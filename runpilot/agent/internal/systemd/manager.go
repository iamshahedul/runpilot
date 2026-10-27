package systemd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"runpilot/agent/internal/cgroups"
	"runpilot/shared/contracts"
)

type Manager struct {
	UnitPrefix  string
	ServicePath string

	// Fallback process simulator for local development / non-systemd environments
	mu        sync.RWMutex
	simCmds   map[string]*exec.Cmd
	simLogs   map[string][]string
	startTime map[string]time.Time
}

func NewManager(unitPrefix, servicePath string) *Manager {
	if unitPrefix == "" {
		unitPrefix = "runpilot-app"
	}
	if servicePath == "" {
		servicePath = "/etc/systemd/system"
	}
	return &Manager{
		UnitPrefix:  unitPrefix,
		ServicePath: servicePath,
		simCmds:     make(map[string]*exec.Cmd),
		simLogs:     make(map[string][]string),
		startTime:   make(map[string]time.Time),
	}
}

func (m *Manager) UnitName(appID string) string {
	return fmt.Sprintf("%s-%s.service", m.UnitPrefix, appID)
}

func (m *Manager) HasSystemd() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

// GenerateUnitFileContent creates the deterministic, hardened systemd service unit
func (m *Manager) GenerateUnitFileContent(appID, unixUser, workingDir, envFile, execStart string, limits contracts.ResourceLimits) string {
	cgroupDirectives := cgroups.GenerateSystemdDirectives(limits)

	return fmt.Sprintf(`[Unit]
Description=RunPilot App %s
After=network.target

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s
EnvironmentFile=%s
ExecStart=%s
Restart=always
RestartSec=5s

# Process Sandboxing & Hardening
ProtectSystem=full
ProtectHome=true
NoNewPrivileges=true
PrivateTmp=true

# Resource Limits (cgroups v2)
%s
[Install]
WantedBy=multi-user.target
`, appID, unixUser, unixUser, workingDir, envFile, execStart, cgroupDirectives)
}

// WriteUnit writes the service unit file to disk
func (m *Manager) WriteUnit(appID, unixUser, workingDir, envFile, execStart string, limits contracts.ResourceLimits) error {
	content := m.GenerateUnitFileContent(appID, unixUser, workingDir, envFile, execStart, limits)
	unitName := m.UnitName(appID)

	if !m.HasSystemd() {
		// In dev/test without systemd, write to local temp or base service path
		_ = os.MkdirAll(m.ServicePath, 0755)
	}

	targetPath := filepath.Join(m.ServicePath, unitName)
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write systemd unit %s: %w", targetPath, err)
	}

	if m.HasSystemd() {
		_ = exec.Command("systemctl", "daemon-reload").Run()
	}

	return nil
}

// Start initiates the service
func (m *Manager) Start(appID, workingDir, execStart string, envVars map[string]string) error {
	unitName := m.UnitName(appID)
	if m.HasSystemd() {
		cmd := exec.Command("systemctl", "start", unitName)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl start %s failed: %s (%w)", unitName, strings.TrimSpace(string(out)), err)
		}
		return nil
	}

	// Local development simulation fallback
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, exists := m.simCmds[appID]; exists && existing.Process != nil {
		_ = existing.Process.Kill()
	}

	parts := strings.Fields(execStart)
	if len(parts) == 0 {
		return fmt.Errorf("invalid execStart command: %s", execStart)
	}

	cmd := exec.Command("bash", "-c", execStart)
	cmd.Dir = workingDir

	env := os.Environ()
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("simulation start failed: %w", err)
	}

	m.simCmds[appID] = cmd
	m.startTime[appID] = time.Now()
	m.simLogs[appID] = append(m.simLogs[appID], fmt.Sprintf("[%s] Process started with PID %d", time.Now().Format(time.RFC3339), cmd.Process.Pid))

	return nil
}

// Stop terminates the service
func (m *Manager) Stop(appID string) error {
	unitName := m.UnitName(appID)
	if m.HasSystemd() {
		cmd := exec.Command("systemctl", "stop", unitName)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl stop %s failed: %s (%w)", unitName, strings.TrimSpace(string(out)), err)
		}
		return nil
	}

	// Local dev fallback
	m.mu.Lock()
	defer m.mu.Unlock()

	if cmd, exists := m.simCmds[appID]; exists && cmd.Process != nil {
		_ = cmd.Process.Kill()
		delete(m.simCmds, appID)
		m.simLogs[appID] = append(m.simLogs[appID], fmt.Sprintf("[%s] Process stopped", time.Now().Format(time.RFC3339)))
	}
	return nil
}

// Restart restarts the service
func (m *Manager) Restart(appID, workingDir, execStart string, envVars map[string]string) error {
	if m.HasSystemd() {
		unitName := m.UnitName(appID)
		cmd := exec.Command("systemctl", "restart", unitName)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl restart %s failed: %s (%w)", unitName, strings.TrimSpace(string(out)), err)
		}
		return nil
	}

	_ = m.Stop(appID)
	return m.Start(appID, workingDir, execStart, envVars)
}

// Status inspects the service state
func (m *Manager) Status(appID string) contracts.AgentProcessStatusResponse {
	resp := contracts.AgentProcessStatusResponse{
		AppID: appID,
		State: contracts.ProcessStopped,
	}

	unitName := m.UnitName(appID)
	if m.HasSystemd() {
		cmd := exec.Command("systemctl", "is-active", unitName)
		out, err := cmd.Output()
		stateStr := strings.TrimSpace(string(out))

		if err == nil && stateStr == "active" {
			resp.State = contracts.ProcessRunning
			// Retrieve MainPID
			pidCmd := exec.Command("systemctl", "show", "-p", "MainPID", "--value", unitName)
			if pidOut, err := pidCmd.Output(); err == nil {
				if pid, err := strconv.Atoi(strings.TrimSpace(string(pidOut))); err == nil {
					resp.PID = pid
				}
			}
		} else {
			resp.State = contracts.ProcessStopped
		}
		return resp
	}

	// Local dev fallback
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cmd, exists := m.simCmds[appID]; exists && cmd.Process != nil {
		// Check if process still active
		if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
			resp.State = contracts.ProcessRunning
			resp.PID = cmd.Process.Pid
			if st, ok := m.startTime[appID]; ok {
				resp.UptimeSec = int64(time.Since(st).Seconds())
			}
		} else {
			resp.State = contracts.ProcessStopped
		}
	}

	return resp
}

// Logs returns recent lines from journalctl or memory buffer
func (m *Manager) Logs(appID string, lines int) []string {
	if lines <= 0 {
		lines = 50
	}

	unitName := m.UnitName(appID)
	if m.HasSystemd() {
		cmd := exec.Command("journalctl", "-u", unitName, "-n", strconv.Itoa(lines), "--no-pager")
		out, err := cmd.Output()
		if err == nil {
			rawLines := strings.Split(string(out), "\n")
			var filtered []string
			for _, l := range rawLines {
				if strings.TrimSpace(l) != "" {
					filtered = append(filtered, l)
				}
			}
			return filtered
		}
	}

	// Fallback
	m.mu.RLock()
	defer m.mu.RUnlock()

	stored, exists := m.simLogs[appID]
	if !exists || len(stored) == 0 {
		return []string{"[RunPilot] No logs available for application"}
	}

	if len(stored) > lines {
		return stored[len(stored)-lines:]
	}
	return stored
}
