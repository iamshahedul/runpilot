package models

import (
	"time"

	"runpilot/shared/contracts"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username,omitempty"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Role         string    `json:"role"` // "admin", "user", "operator"
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Package struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	MemoryMax  string    `json:"memory_max"`
	CPUQuota   string    `json:"cpu_quota"`
	TasksMax   int       `json:"tasks_max"`
	DiskQuota  string    `json:"disk_quota"`
	MaxApps    int       `json:"max_apps"`
	MaxDomains int       `json:"max_domains"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Team struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	OwnerID   int64     `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Server struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Hostname  string    `json:"hostname"`
	IPAddress string    `json:"ip_address"`
	Status    string    `json:"status"` // "active", "offline"
	OSInfo    string    `json:"os_info"`
	CPUCores  int       `json:"cpu_cores"`
	MemoryMB  int       `json:"memory_mb"`
	DiskGB    int       `json:"disk_gb"`
	CreatedAt time.Time `json:"created_at"`
}

type Application struct {
	ID             string                   `json:"id"`
	Name           string                   `json:"name"`
	OwnerID        int64                    `json:"owner_id"`
	TeamID         *int64                   `json:"team_id,omitempty"`
	ServerID       int64                    `json:"server_id"`
	RuntimeType    contracts.RuntimeType    `json:"runtime_type"`
	RuntimeVersion string                   `json:"runtime_version"`
	GitRepo        string                   `json:"git_repo"`
	GitBranch      string                   `json:"git_branch"`
	Port           int                      `json:"port"`
	BuildCommand   string                   `json:"build_command"`
	StartCommand   string                   `json:"start_command"`
	HealthPath     string                   `json:"health_path"`
	AssignedUnix   string                   `json:"assigned_unix_user"`
	Status         contracts.ProcessState   `json:"status"`
	CurrentRelease string                   `json:"current_release,omitempty"`
	Package        string                   `json:"package,omitempty"`
	ResourceLimits contracts.ResourceLimits `json:"resource_limits"`
	Domains        []string                 `json:"domains,omitempty"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
}

type Deployment struct {
	ID           string                    `json:"id"`
	AppID        string                    `json:"app_id"`
	CommitHash   string                    `json:"commit_hash"`
	CommitMsg    string                    `json:"commit_message"`
	State        contracts.DeploymentState `json:"state"`
	ErrorMessage string                    `json:"error_message,omitempty"`
	TriggeredBy  string                    `json:"triggered_by"`
	DurationSec  int64                     `json:"duration_sec"`
	CreatedAt    time.Time                 `json:"created_at"`
	FinishedAt   *time.Time                `json:"finished_at,omitempty"`
}

type AuditEvent struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id,omitempty"`
	UserEmail string    `json:"user_email,omitempty"`
	AppID     string    `json:"app_id,omitempty"`
	Action    string    `json:"action"`
	IPAddress string    `json:"ip_address,omitempty"`
	Details   string    `json:"details,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
