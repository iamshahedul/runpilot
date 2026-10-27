package integration

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func TestCLIWithLiveServerAndAgent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runpilot-cli-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	binDir := filepath.Join("..", "..", "bin")
	serverBin := filepath.Join(binDir, "runpilot-server")
	agentBin := filepath.Join(binDir, "runpilot-agent")
	cliBin := filepath.Join(binDir, "runpilot")

	// Ensure binaries exist
	for _, b := range []string{serverBin, agentBin, cliBin} {
		if _, err := os.Stat(b); err != nil {
			t.Fatalf("Required binary %s not found (run make build first): %v", b, err)
		}
	}

	serverPort, _ := getFreePort()
	agentPort, _ := getFreePort()

	// 1. Write Agent Config
	agentConfigPath := filepath.Join(tempDir, "agent.json")
	agentConfigContent := fmt.Sprintf(`{
		"agent": {
			"host": "127.0.0.1",
			"port": %d,
			"secret_key": "test-agent-secret-key-32bytes!!",
			"data_dir": "%s",
			"apps_dir": "%s/apps",
			"runtimes_dir": "%s/runtimes",
			"log_dir": "%s/logs",
			"nginx_sites_dir": "%s/nginx",
			"systemd_unit_prefix": "runpilot-app",
			"systemd_service_path": "%s/systemd"
		}
	}`, agentPort, tempDir, tempDir, tempDir, tempDir, tempDir, tempDir)
	_ = os.WriteFile(agentConfigPath, []byte(agentConfigContent), 0600)

	// 2. Write Server Config
	serverConfigPath := filepath.Join(tempDir, "server.json")
	serverConfigContent := fmt.Sprintf(`{
		"server": {
			"host": "127.0.0.1",
			"port": %d,
			"web_root": "../../control-plane/web",
			"jwt_secret": "jwt-secret-key-for-test-32bytes!",
			"master_key": "master-aes-key-for-test-32bytes!",
			"data_dir": "%s"
		},
		"database": {
			"driver": "sqlite",
			"sqlite_path": "%s/test.db"
		},
		"agent": {
			"endpoint": "http://127.0.0.1:%d",
			"secret_key": "test-agent-secret-key-32bytes!!"
		}
	}`, serverPort, tempDir, tempDir, agentPort)
	_ = os.WriteFile(serverConfigPath, []byte(serverConfigContent), 0600)

	// 3. Start Agent process
	agentCmd := exec.Command(agentBin, "-config", agentConfigPath)
	if err := agentCmd.Start(); err != nil {
		t.Fatalf("Failed to start runpilot-agent: %v", err)
	}
	defer func() {
		if agentCmd.Process != nil {
			_ = agentCmd.Process.Kill()
		}
	}()

	// 4. Start Server process
	migPath := filepath.Join("..", "..", "control-plane", "migrations", "001_initial_schema.sql")
	serverCmd := exec.Command(serverBin, "-config", serverConfigPath, "-migrations", migPath)
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start runpilot-server: %v", err)
	}
	defer func() {
		if serverCmd.Process != nil {
			_ = serverCmd.Process.Kill()
		}
	}()

	// Wait up to 3 seconds for server to be responsive
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", serverPort)
	responsive := false
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get(serverURL + "/api/v1/servers")
		if err == nil {
			resp.Body.Close()
			responsive = true
			break
		}
	}

	if !responsive {
		t.Fatalf("Server failed to become responsive at %s", serverURL)
	}

	// 5. Test CLI 'version'
	verCmd := exec.Command(cliBin, "version")
	verOut, err := verCmd.CombinedOutput()
	if err != nil || !strings.Contains(string(verOut), "RunPilot CLI v1.0.0") {
		t.Fatalf("CLI version command failed: %s (%v)", string(verOut), err)
	}

	// 6. Test CLI 'login'
	// Use fake HOME so credentials are saved in tempDir
	cliEnv := append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))

	loginCmd := exec.Command(cliBin, "login")
	loginCmd.Env = cliEnv
	loginCmd.Stdin = strings.NewReader(fmt.Sprintf("%s\nadmin@runpilot.local\nRunPilot2026!\n", serverURL))
	var loginOut bytes.Buffer
	loginCmd.Stdout = &loginOut
	loginCmd.Stderr = &loginOut
	if err := loginCmd.Run(); err != nil {
		t.Fatalf("CLI login command failed: %s (%v)", loginOut.String(), err)
	}

	if !strings.Contains(loginOut.String(), "Successfully authenticated") {
		t.Errorf("Expected login success message, got: %s", loginOut.String())
	}

	// 7. Test CLI 'apps create'
	createCmd := exec.Command(cliBin, "apps", "create",
		"--id", "cli-test-app",
		"--name", "CLI Node App",
		"--runtime", "nodejs",
		"--version", "20.x",
		"--repo", "https://github.com/example/repo.git",
		"--port", "3000",
		"--start", "node server.js",
	)
	createCmd.Env = cliEnv
	createOut, err := createCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI apps create failed: %s (%v)", string(createOut), err)
	}
	if !strings.Contains(string(createOut), "Created application: CLI Node App") {
		t.Errorf("Expected creation confirmation, got: %s", string(createOut))
	}

	// 8. Test CLI 'apps list'
	listCmd := exec.Command(cliBin, "apps", "list")
	listCmd.Env = cliEnv
	listOut, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI apps list failed: %s (%v)", string(listOut), err)
	}
	if !strings.Contains(string(listOut), "cli-test-app") || !strings.Contains(string(listOut), "CLI Node App") {
		t.Errorf("Listed apps missing created app: %s", string(listOut))
	}

	// 9. Test CLI 'status'
	statusCmd := exec.Command(cliBin, "status", "cli-test-app")
	statusCmd.Env = cliEnv
	statusOut, err := statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI status command failed: %s (%v)", string(statusOut), err)
	}
	if !strings.Contains(string(statusOut), "CLI Node App") || !strings.Contains(string(statusOut), "runpilot-app-cli-test-app") {
		t.Errorf("Status output missing expected fields: %s", string(statusOut))
	}

	// 10. Test CLI 'servers list'
	srvListCmd := exec.Command(cliBin, "servers", "list")
	srvListCmd.Env = cliEnv
	srvListOut, err := srvListCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI servers list failed: %s (%v)", string(srvListOut), err)
	}
	if !strings.Contains(string(srvListOut), "Primary Node") {
		t.Errorf("Server list missing Primary Node: %s", string(srvListOut))
	}

	fmt.Println("✓ Full CLI + Control Plane + Host Agent integration verified!")
}
