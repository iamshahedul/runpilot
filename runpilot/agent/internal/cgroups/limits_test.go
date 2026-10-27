package cgroups

import (
	"strings"
	"testing"

	"runpilot/shared/contracts"
)

func TestValidateAndSanitizeLimits(t *testing.T) {
	// Normal valid limits
	limits := contracts.ResourceLimits{
		CPUQuota:  "150%",
		MemoryMax: "1G",
		TasksMax:  256,
	}

	clean := ValidateAndSanitizeLimits(limits)
	if clean.CPUQuota != "150%" || clean.MemoryMax != "1G" || clean.TasksMax != 256 {
		t.Errorf("Limits mutated unexpectedly: %+v", clean)
	}

	// Malicious or invalid values should fall back to safe defaults
	invalidLimits := contracts.ResourceLimits{
		CPUQuota:  "999999% ; rm -rf /",
		MemoryMax: "invalid_ram",
		TasksMax:  -10,
	}

	sanitized := ValidateAndSanitizeLimits(invalidLimits)
	if sanitized.CPUQuota != "100%" {
		t.Errorf("Expected fallback CPU 100%%, got %s", sanitized.CPUQuota)
	}
	if sanitized.MemoryMax != "512M" {
		t.Errorf("Expected fallback Memory 512M, got %s", sanitized.MemoryMax)
	}
	if sanitized.TasksMax != 128 {
		t.Errorf("Expected fallback TasksMax 128, got %d", sanitized.TasksMax)
	}

	directives := GenerateSystemdDirectives(sanitized)
	if !strings.Contains(directives, "CPUQuota=100%") || !strings.Contains(directives, "MemoryMax=512M") {
		t.Errorf("Generated directives missing expected values: %s", directives)
	}
}
