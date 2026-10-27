package contracts

import (
	"time"
)

// RuntimeType defines the execution ecosystem
type RuntimeType string

const (
	RuntimeNodeJS RuntimeType = "nodejs"
	RuntimePython RuntimeType = "python"
)

// DeploymentState represents the lifecycle of a deployment job
type DeploymentState string

const (
	DeployStateQueued      DeploymentState = "queued"
	DeployStateFetching    DeploymentState = "fetching"
	DeployStateBuilding    DeploymentState = "building"
	DeployStateConfiguring DeploymentState = "configuring"
	DeployStateStarting    DeploymentState = "starting"
	DeployStateVerifying   DeploymentState = "verifying"
	DeployStateSuccessful  DeploymentState = "successful"
	DeployStateFailed      DeploymentState = "failed"
	DeployStateRolledBack  DeploymentState = "rolled_back"
)

// ProcessState represents current service execution status
type ProcessState string

const (
	ProcessRunning  ProcessState = "running"
	ProcessStopped  ProcessState = "stopped"
	ProcessStarting ProcessState = "starting"
	ProcessFailed   ProcessState = "failed"
	ProcessUnknown  ProcessState = "unknown"
)

// ResourceLimits represents cgroups v2 / systemd quotas
type ResourceLimits struct {
	CPUQuota  string `json:"cpu_quota"`  // e.g. "100%", "200%"
	MemoryMax string `json:"memory_max"` // e.g. "512M", "1G"
	TasksMax  int64  `json:"tasks_max"`  // e.g. 128
	DiskQuota string `json:"disk_quota"` // e.g. "10G"
}

// App represents an application managed by RunPilot
type App struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	OwnerID        int64             `json:"owner_id"`
	TeamID         *int64            `json:"team_id,omitempty"`
	ServerID       int64             `json:"server_id"`
	RuntimeType    RuntimeType       `json:"runtime_type"`
	RuntimeVersion string            `json:"runtime_version"`
	GitRepo        string            `json:"git_repo"`
	GitBranch      string            `json:"git_branch"`
	Port           int               `json:"port"`
	BuildCmd       string            `json:"build_command"`
	StartCmd       string            `json:"start_command"`
	HealthPath     string            `json:"health_path"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	ResourceLimits ResourceLimits    `json:"resource_limits"`
	Status         ProcessState      `json:"status"`
	CurrentRelease string            `json:"current_release,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// Deployment records a single deployment attempt
type Deployment struct {
	ID           string          `json:"id"`
	AppID        string          `json:"app_id"`
	CommitHash   string          `json:"commit_hash"`
	CommitMsg    string          `json:"commit_message"`
	State        DeploymentState `json:"state"`
	ErrorMessage string          `json:"error_message,omitempty"`
	TriggeredBy  string          `json:"triggered_by"`
	BuildOutput  string          `json:"build_output,omitempty"`
	DurationSec  int64           `json:"duration_sec"`
	CreatedAt    time.Time       `json:"created_at"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty"`
}

// Domain represents a domain name assigned to an application
type Domain struct {
	ID        int64     `json:"id"`
	AppID     string    `json:"app_id"`
	Domain    string    `json:"domain"`
	SSLEnabled bool     `json:"ssl_enabled"`
	SSLStatus  string   `json:"ssl_status"` // e.g. "active", "pending", "none"
	CreatedAt time.Time `json:"created_at"`
}

// BackgroundWorker represents a daemon process managed alongside an app
type BackgroundWorker struct {
	ID        string       `json:"id"`
	AppID     string       `json:"app_id"`
	Name      string       `json:"name"`
	Command   string       `json:"command"`
	Instances int          `json:"instances"`
	Status    ProcessState `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
}

// ScheduledJob represents an application cron job
type ScheduledJob struct {
	ID         string    `json:"id"`
	AppID      string    `json:"app_id"`
	Name       string    `json:"name"`
	Schedule   string    `json:"schedule"` // e.g. "0 * * * *"
	Command    string    `json:"command"`
	Enabled    bool      `json:"enabled"`
	LastRun    *time.Time `json:"last_run,omitempty"`
	LastStatus string    `json:"last_status,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// -----------------------------------------------------------------------------
// Host Agent Communication Contracts
// -----------------------------------------------------------------------------

// AgentDeployRequest represents an instruction from Control Plane to Agent
type AgentDeployRequest struct {
	AppID          string            `json:"app_id"`
	ReleaseID      string            `json:"release_id"`
	RuntimeType    RuntimeType       `json:"runtime_type"`
	RuntimeVersion string            `json:"runtime_version"`
	GitRepo        string            `json:"git_repo"`
	GitBranch      string            `json:"git_branch"`
	CommitHash     string            `json:"commit_hash"`
	Port           int               `json:"port"`
	BuildCmd       string            `json:"build_command"`
	StartCmd       string            `json:"start_command"`
	HealthPath     string            `json:"health_path"`
	EnvVars        map[string]string `json:"env_vars"`
	ResourceLimits ResourceLimits    `json:"resource_limits"`
}

// AgentDeployResponse is returned by Agent after building & staging
type AgentDeployResponse struct {
	Success      bool   `json:"success"`
	ReleaseID    string `json:"release_id"`
	BuildOutput  string `json:"build_output"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// AgentProcessActionRequest sends lifecycle action to Agent
type AgentProcessActionRequest struct {
	AppID  string `json:"app_id"`
	Action string `json:"action"` // start, stop, restart, status
}

// AgentProcessStatusResponse describes current process state
type AgentProcessStatusResponse struct {
	AppID        string       `json:"app_id"`
	State        ProcessState `json:"state"`
	PID          int          `json:"pid"`
	MemoryBytes  uint64       `json:"memory_bytes"`
	CPUPercent   float64      `json:"cpu_percent"`
	UptimeSec    int64        `json:"uptime_sec"`
	LastExitCode int          `json:"last_exit_code"`
}

// AgentLogsRequest requests recent process logs
type AgentLogsRequest struct {
	AppID string `json:"app_id"`
	Lines int    `json:"lines"`
}

// AgentLogsResponse returns logs
type AgentLogsResponse struct {
	AppID string   `json:"app_id"`
	Lines []string `json:"lines"`
}

// AgentNginxConfigRequest instructs Agent to create/update Nginx reverse proxy
type AgentNginxConfigRequest struct {
	AppID            string   `json:"app_id"`
	Domains          []string `json:"domains"`
	UpstreamPort     int      `json:"upstream_port"`
	SSLEnabled       bool     `json:"ssl_enabled"`
	SSLCertPath      string   `json:"ssl_cert_path,omitempty"`
	SSLKeyPath       string   `json:"ssl_key_path,omitempty"`
	WebSocketEnabled bool     `json:"websocket_enabled"`
}

// AgentServerMetrics returns host system usage metrics
type AgentServerMetrics struct {
	Hostname       string  `json:"hostname"`
	OS             string  `json:"os"`
	Kernel         string  `json:"kernel"`
	Arch           string  `json:"arch"`
	UptimeSec      uint64  `json:"uptime_sec"`
	CPUCount       int     `json:"cpu_count"`
	CPUUsagePct    float64 `json:"cpu_usage_pct"`
	MemoryTotalMB  uint64  `json:"memory_total_mb"`
	MemoryUsedMB   uint64  `json:"memory_used_mb"`
	MemoryFreeMB   uint64  `json:"memory_free_mb"`
	DiskTotalGB    uint64  `json:"disk_total_gb"`
	DiskUsedGB     uint64  `json:"disk_used_gb"`
	DiskFreeGB     uint64  `json:"disk_free_gb"`
	ActiveApps     int     `json:"active_apps"`
}

// GenericResponse for standard API responses
type GenericResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
