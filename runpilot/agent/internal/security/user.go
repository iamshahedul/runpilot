package security

import (
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var (
	ErrInvalidAppID   = errors.New("invalid application id: must contain only alphanumeric, hyphens, and underscores")
	ErrPathTraversal  = errors.New("potential path traversal detected")
	validAppIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
)

// ValidateAppID ensures the application ID is safe for filenames and system users
func ValidateAppID(appID string) error {
	if len(appID) == 0 || len(appID) > 40 {
		return ErrInvalidAppID
	}
	if !validAppIDPattern.MatchString(appID) {
		return ErrInvalidAppID
	}
	return nil
}

// AppUnixUsername derives the unprivileged Unix username for an application
func AppUnixUsername(prefix, appID string) string {
	if prefix == "" {
		prefix = "runpilot-app"
	}
	// Maximum Linux username length is usually 32 characters
	sanitized := fmt.Sprintf("%s-%s", prefix, appID)
	if len(sanitized) > 31 {
		sanitized = sanitized[:31]
	}
	return sanitized
}

// EnsureAppUser provisions a dedicated unprivileged system account on Linux
func EnsureAppUser(username string) error {
	if runtime.GOOS != "linux" {
		// In dev/test on macOS, do not attempt Linux useradd
		return nil
	}

	// Check if user already exists
	_, err := user.Lookup(username)
	if err == nil {
		return nil // User already exists
	}

	// Create system user with no login shell and no home directory
	cmd := exec.Command("useradd", "--system", "--no-create-home", "--shell", "/bin/false", username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If group exists or already present, ignore
		if strings.Contains(string(output), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to create system user %s: %s (%w)", username, strings.TrimSpace(string(output)), err)
	}

	return nil
}

// SafePath joins and verifies that target resides strictly within allowedBase without escaping
func SafePath(allowedBase, subPath string) (string, error) {
	if filepath.IsAbs(subPath) || strings.HasPrefix(subPath, "/") || strings.HasPrefix(subPath, "\\") {
		return "", fmt.Errorf("%w: absolute path %s not permitted", ErrPathTraversal, subPath)
	}

	for _, part := range strings.Split(filepath.ToSlash(subPath), "/") {
		if part == ".." {
			return "", fmt.Errorf("%w: parent path traversal component '..' not permitted: %s", ErrPathTraversal, subPath)
		}
	}

	cleanBase := filepath.Clean(allowedBase)
	target := filepath.Join(cleanBase, subPath)
	cleanTarget := filepath.Clean(target)

	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil || strings.HasPrefix(rel, "..") || (rel == "." && cleanTarget != cleanBase) {
		return "", fmt.Errorf("%w: path %s escapes %s", ErrPathTraversal, subPath, allowedBase)
	}

	return cleanTarget, nil
}
