package nodejs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type PackageManager string

const (
	PkgNpm  PackageManager = "npm"
	PkgPnpm PackageManager = "pnpm"
	PkgYarn PackageManager = "yarn"
)

type NodeRuntimeManager struct {
	DefaultVersion string
	RuntimesBaseDir string
}

func NewNodeRuntimeManager(runtimesDir string) *NodeRuntimeManager {
	return &NodeRuntimeManager{
		DefaultVersion:  "20.x",
		RuntimesBaseDir: runtimesDir,
	}
}

// SupportedVersions returns the supported Node.js release lines
func (m *NodeRuntimeManager) SupportedVersions() []string {
	return []string{"18.x", "20.x", "22.x"}
}

// DetectPackageManager inspects lockfiles in the release workspace
func (m *NodeRuntimeManager) DetectPackageManager(releaseDir string) PackageManager {
	if _, err := os.Stat(filepath.Join(releaseDir, "pnpm-lock.yaml")); err == nil {
		return PkgPnpm
	}
	if _, err := os.Stat(filepath.Join(releaseDir, "yarn.lock")); err == nil {
		return PkgYarn
	}
	return PkgNpm
}

// InspectPackageJSON reads scripts and entry point from package.json
func (m *NodeRuntimeManager) InspectPackageJSON(releaseDir string) (mainFile string, hasStartScript bool, hasBuildScript bool, err error) {
	pkgPath := filepath.Join(releaseDir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return "", false, false, err
	}

	var pkg struct {
		Main    string            `json:"main"`
		Scripts map[string]string `json:"scripts"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", false, false, err
	}

	mainFile = pkg.Main
	if pkg.Scripts != nil {
		_, hasStartScript = pkg.Scripts["start"]
		_, hasBuildScript = pkg.Scripts["build"]
	}

	return mainFile, hasStartScript, hasBuildScript, nil
}

// ResolveStartCommand determines the appropriate start command
func (m *NodeRuntimeManager) ResolveStartCommand(releaseDir string, userCommand string) string {
	if strings.TrimSpace(userCommand) != "" {
		return userCommand
	}

	mainFile, hasStart, _, err := m.InspectPackageJSON(releaseDir)
	if err == nil {
		if hasStart {
			return "npm start"
		}
		if mainFile != "" {
			return fmt.Sprintf("node %s", mainFile)
		}
	}

	// Fallback to common convention files
	for _, candidate := range []string{"server.js", "app.js", "index.js", "dist/index.js", "src/index.js"} {
		if _, err := os.Stat(filepath.Join(releaseDir, candidate)); err == nil {
			return fmt.Sprintf("node %s", candidate)
		}
	}

	return "node server.js"
}

// BuildRelease runs the build command in the unprivileged release directory
func (m *NodeRuntimeManager) BuildRelease(ctx context.Context, releaseDir string, buildCommand string, envVars map[string]string) (string, error) {
	// If no explicit build command, formulate default based on package manager
	if strings.TrimSpace(buildCommand) == "" {
		pkgMgr := m.DetectPackageManager(releaseDir)
		_, _, hasBuild, _ := m.InspectPackageJSON(releaseDir)

		switch pkgMgr {
		case PkgPnpm:
			if hasBuild {
				buildCommand = "pnpm install && pnpm run build"
			} else {
				buildCommand = "pnpm install"
			}
		case PkgYarn:
			if hasBuild {
				buildCommand = "yarn install && yarn run build"
			} else {
				buildCommand = "yarn install"
			}
		default:
			if hasBuild {
				buildCommand = "npm install && npm run build"
			} else {
				buildCommand = "npm install"
			}
		}
	}

	// Prepare safe execution context with timeout
	buildCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(buildCtx, "bash", "-c", buildCommand)
	cmd.Dir = releaseDir

	// Setup clean environment with app variables
	env := os.Environ()
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("build failed: %s (%w)", string(output), err)
	}

	return string(output), nil
}
