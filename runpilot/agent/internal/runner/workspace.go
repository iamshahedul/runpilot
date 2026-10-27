package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"runpilot/agent/internal/security"
)

type Workspace struct {
	BaseDir string
}

func NewWorkspace(baseDir string) *Workspace {
	return &Workspace{BaseDir: baseDir}
}

// AppDir returns the root path for an application
func (w *Workspace) AppDir(appID string) (string, error) {
	if err := security.ValidateAppID(appID); err != nil {
		return "", err
	}
	return security.SafePath(w.BaseDir, appID)
}

// SetupAppDirectories prepares the atomic release structure
func (w *Workspace) SetupAppDirectories(appID string) error {
	appRoot, err := w.AppDir(appID)
	if err != nil {
		return err
	}

	dirs := []string{
		filepath.Join(appRoot, "releases"),
		filepath.Join(appRoot, "shared"),
		filepath.Join(appRoot, "shared", "logs"),
		filepath.Join(appRoot, "shared", "uploads"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// WriteEnvironmentFile writes environment variables to shared/.env with 0600 permissions
func (w *Workspace) WriteEnvironmentFile(appID string, envVars map[string]string) error {
	appRoot, err := w.AppDir(appID)
	if err != nil {
		return err
	}

	envPath := filepath.Join(appRoot, "shared", ".env")
	var sb strings.Builder

	for k, v := range envVars {
		// Escape or format safely
		escaped := strings.ReplaceAll(v, "\n", "\\n")
		sb.WriteString(fmt.Sprintf("%s=\"%s\"\n", k, escaped))
	}

	if err := os.WriteFile(envPath, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("failed to write environment file %s: %w", envPath, err)
	}

	return nil
}

// PrepareReleaseDirectory creates a unique release folder
func (w *Workspace) PrepareReleaseDirectory(appID, releaseID string) (string, error) {
	appRoot, err := w.AppDir(appID)
	if err != nil {
		return "", err
	}

	relDir := filepath.Join(appRoot, "releases", releaseID)
	if err := os.MkdirAll(relDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create release directory %s: %w", relDir, err)
	}

	return relDir, nil
}

// ActivateRelease atomically switches the 'current' symlink to the specified release
func (w *Workspace) ActivateRelease(appID, releaseID string) error {
	appRoot, err := w.AppDir(appID)
	if err != nil {
		return err
	}

	targetReleaseDir := filepath.Join(appRoot, "releases", releaseID)
	if _, err := os.Stat(targetReleaseDir); err != nil {
		return fmt.Errorf("target release directory %s does not exist: %w", targetReleaseDir, err)
	}

	currentSymlink := filepath.Join(appRoot, "current")
	tmpSymlink := filepath.Join(appRoot, fmt.Sprintf("current_tmp_%s", releaseID))

	// Remove old tmp symlink if present
	_ = os.Remove(tmpSymlink)

	// Create relative symlink: releases/<releaseID>
	relTarget := filepath.Join("releases", releaseID)
	if err := os.Symlink(relTarget, tmpSymlink); err != nil {
		return fmt.Errorf("failed to create release symlink: %w", err)
	}

	// Atomic rename replaces the current symlink instantaneously
	if err := os.Rename(tmpSymlink, currentSymlink); err != nil {
		_ = os.Remove(tmpSymlink)
		return fmt.Errorf("failed to atomically switch current symlink: %w", err)
	}

	return nil
}

// ListReleases returns all available release directories sorted chronologically
func (w *Workspace) ListReleases(appID string) ([]string, error) {
	appRoot, err := w.AppDir(appID)
	if err != nil {
		return nil, err
	}

	releasesRoot := filepath.Join(appRoot, "releases")
	entries, err := os.ReadDir(releasesRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var releases []string
	for _, entry := range entries {
		if entry.IsDir() {
			releases = append(releases, entry.Name())
		}
	}

	sort.Strings(releases)
	return releases, nil
}

// RollbackRelease points current to a previous release if valid
func (w *Workspace) RollbackRelease(appID, targetReleaseID string) error {
	releases, err := w.ListReleases(appID)
	if err != nil {
		return err
	}

	found := false
	for _, r := range releases {
		if r == targetReleaseID {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("cannot rollback: release %s not found in application history", targetReleaseID)
	}

	return w.ActivateRelease(appID, targetReleaseID)
}
