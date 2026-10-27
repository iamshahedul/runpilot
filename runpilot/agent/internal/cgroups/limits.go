package cgroups

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"runpilot/shared/contracts"
)

var (
	cpuPattern    = regexp.MustCompile(`^[1-9][0-9]{0,3}%$`)
	memoryPattern = regexp.MustCompile(`^[1-9][0-9]{0,4}[KMGkmg]$`)
)

// IsCGroupsV2Supported checks if unified cgroups v2 controllers are available
func IsCGroupsV2Supported() bool {
	_, err := os.Stat("/sys/fs/cgroup/cgroup.controllers")
	return err == nil
}

// ValidateAndSanitizeLimits ensures limits adhere to safe formats
func ValidateAndSanitizeLimits(limits contracts.ResourceLimits) contracts.ResourceLimits {
	sanitized := limits

	// Default or validate CPU Quota
	if sanitized.CPUQuota == "" || !cpuPattern.MatchString(sanitized.CPUQuota) {
		sanitized.CPUQuota = "100%"
	}

	// Default or validate Memory Limit
	if sanitized.MemoryMax == "" || !memoryPattern.MatchString(sanitized.MemoryMax) {
		sanitized.MemoryMax = "512M"
	} else {
		sanitized.MemoryMax = strings.ToUpper(sanitized.MemoryMax)
	}

	// Tasks limit
	if sanitized.TasksMax <= 0 || sanitized.TasksMax > 4096 {
		sanitized.TasksMax = 128
	}

	return sanitized
}

// GenerateSystemdDirectives renders cgroups v2 resource directives for a service unit
func GenerateSystemdDirectives(limits contracts.ResourceLimits) string {
	clean := ValidateAndSanitizeLimits(limits)
	return fmt.Sprintf("CPUQuota=%s\nMemoryMax=%s\nTasksMax=%d\n",
		clean.CPUQuota,
		clean.MemoryMax,
		clean.TasksMax,
	)
}
