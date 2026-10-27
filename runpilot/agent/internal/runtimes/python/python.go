package python

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type PythonRuntimeManager struct {
	DefaultVersion string
	RuntimesBaseDir string
}

func NewPythonRuntimeManager(runtimesDir string) *PythonRuntimeManager {
	return &PythonRuntimeManager{
		DefaultVersion:  "3.11",
		RuntimesBaseDir: runtimesDir,
	}
}

// SupportedVersions returns supported Python versions
func (m *PythonRuntimeManager) SupportedVersions() []string {
	return []string{"3.10", "3.11", "3.12"}
}

// ResolvePythonBinary finds the appropriate system or installed Python binary
func (m *PythonRuntimeManager) ResolvePythonBinary(version string) string {
	if version != "" {
		// Check for specific version binary e.g. python3.11
		vBin := fmt.Sprintf("python%s", version)
		if path, err := exec.LookPath(vBin); err == nil {
			return path
		}
	}
	if path, err := exec.LookPath("python3"); err == nil {
		return path
	}
	return "python3"
}

// CreateVirtualEnvironment provisions an isolated .venv in the release directory
func (m *PythonRuntimeManager) CreateVirtualEnvironment(ctx context.Context, releaseDir string, version string) error {
	venvDir := filepath.Join(releaseDir, ".venv")
	if _, err := os.Stat(venvDir); err == nil {
		return nil // Already created
	}

	pyBin := m.ResolvePythonBinary(version)
	cmd := exec.CommandContext(ctx, pyBin, "-m", "venv", ".venv")
	cmd.Dir = releaseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create virtual environment: %s (%w)", string(output), err)
	}

	return nil
}

// BuildRelease creates venv and installs dependencies from requirements.txt or pyproject.toml
func (m *PythonRuntimeManager) BuildRelease(ctx context.Context, releaseDir string, buildCommand string, envVars map[string]string) (string, error) {
	buildCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// 1. Ensure venv exists
	if err := m.CreateVirtualEnvironment(buildCtx, releaseDir, m.DefaultVersion); err != nil {
		return "", err
	}

	pipPath := filepath.Join(releaseDir, ".venv", "bin", "pip")

	var logBuilder strings.Builder

	// 2. Install dependencies if requirements.txt exists and no custom build command
	if strings.TrimSpace(buildCommand) == "" {
		reqPath := filepath.Join(releaseDir, "requirements.txt")
		pyprojectPath := filepath.Join(releaseDir, "pyproject.toml")

		if _, err := os.Stat(reqPath); err == nil {
			buildCommand = fmt.Sprintf("%s install --no-cache-dir -r requirements.txt", pipPath)
		} else if _, err := os.Stat(pyprojectPath); err == nil {
			buildCommand = fmt.Sprintf("%s install --no-cache-dir .", pipPath)
		}
	}

	if strings.TrimSpace(buildCommand) != "" {
		cmd := exec.CommandContext(buildCtx, "bash", "-c", buildCommand)
		cmd.Dir = releaseDir

		env := os.Environ()
		// Prepend .venv/bin to PATH for build execution
		venvBin := filepath.Join(releaseDir, ".venv", "bin")
		env = append(env, fmt.Sprintf("PATH=%s:%s", venvBin, os.Getenv("PATH")))
		env = append(env, fmt.Sprintf("VIRTUAL_ENV=%s", filepath.Join(releaseDir, ".venv")))

		for k, v := range envVars {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env

		out, err := cmd.CombinedOutput()
		logBuilder.Write(out)
		if err != nil {
			return logBuilder.String(), fmt.Errorf("python build failed: %s (%w)", logBuilder.String(), err)
		}
	}

	return logBuilder.String(), nil
}

// ResolveStartCommand formulates the entry command for WSGI/ASGI or scripts
func (m *PythonRuntimeManager) ResolveStartCommand(releaseDir string, userCommand string, port int) string {
	if strings.TrimSpace(userCommand) != "" {
		return userCommand
	}

	venvBin := filepath.Join(releaseDir, ".venv", "bin")
	gunicornPath := filepath.Join(venvBin, "gunicorn")
	uvicornPath := filepath.Join(venvBin, "uvicorn")

	// Check for ASGI (FastAPI)
	if _, err := os.Stat(uvicornPath); err == nil {
		for _, f := range []string{"main.py", "app.py", "api.py"} {
			if _, err := os.Stat(filepath.Join(releaseDir, f)); err == nil {
				mod := strings.TrimSuffix(f, ".py")
				return fmt.Sprintf(".venv/bin/uvicorn %s:app --host 127.0.0.1 --port %d", mod, port)
			}
		}
	}

	// Check for WSGI (Flask / Django)
	if _, err := os.Stat(gunicornPath); err == nil {
		for _, f := range []string{"wsgi.py", "app.py", "wsgi:application", "main:app"} {
			if _, err := os.Stat(filepath.Join(releaseDir, f)); err == nil {
				mod := strings.TrimSuffix(f, ".py")
				return fmt.Sprintf(".venv/bin/gunicorn --bind 127.0.0.1:%d %s:app", port, mod)
			}
		}
	}

	// Fallback to python entry file
	for _, f := range []string{"main.py", "app.py", "server.py", "manage.py"} {
		if _, err := os.Stat(filepath.Join(releaseDir, f)); err == nil {
			return fmt.Sprintf(".venv/bin/python %s", f)
		}
	}

	return ".venv/bin/python main.py"
}
