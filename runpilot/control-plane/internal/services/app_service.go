package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"runpilot/control-plane/internal/config"
	"runpilot/control-plane/internal/db"
	"runpilot/control-plane/internal/models"
	"runpilot/shared/contracts"
	"runpilot/shared/crypto"
)

var (
	ErrAppNotFound      = errors.New("application not found")
	ErrPermissionDenied = errors.New("permission denied: you do not own this application")
	ErrDeployInProgress = errors.New("a deployment is already in progress for this application")
)

type AppService struct {
	db          *db.DB
	cfg         *config.Config
	agentClient *AgentClient
	deployLocks sync.Map // map[string]*sync.Mutex
}

func NewAppService(database *db.DB, cfg *config.Config, agentClient *AgentClient) *AppService {
	return &AppService{
		db:          database,
		cfg:         cfg,
		agentClient: agentClient,
	}
}

func (s *AppService) getLock(appID string) *sync.Mutex {
	lock, _ := s.deployLocks.LoadOrStore(appID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// CreateApp creates a new application and assigns default limits
func (s *AppService) CreateApp(ownerID int64, req models.Application) (*models.Application, error) {
	if req.ID == "" {
		req.ID = fmt.Sprintf("app-%d", time.Now().UnixNano()%1000000)
	}
	req.ID = strings.ToLower(strings.TrimSpace(req.ID))
	req.OwnerID = ownerID
	req.AssignedUnix = fmt.Sprintf("runpilot-app-%s", req.ID)
	req.Status = contracts.ProcessStopped

	if req.Port <= 0 {
		req.Port = 3000
	}
	if req.RuntimeType == "" {
		req.RuntimeType = contracts.RuntimeNodeJS
	}
	if req.RuntimeVersion == "" {
		if req.RuntimeType == contracts.RuntimeNodeJS {
			req.RuntimeVersion = "20.x"
		} else {
			req.RuntimeVersion = "3.11"
		}
	}
	if req.GitBranch == "" {
		req.GitBranch = "main"
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Insert Application
	_, err = tx.Exec(`
		INSERT INTO applications (id, name, owner_id, team_id, server_id, runtime_type, runtime_version, git_repo, git_branch, port, build_command, start_command, health_path, assigned_unix_user, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, req.ID, req.Name, req.OwnerID, req.TeamID, 1, string(req.RuntimeType), req.RuntimeVersion, req.GitRepo, req.GitBranch, req.Port, req.BuildCommand, req.StartCommand, req.HealthPath, req.AssignedUnix, string(req.Status))
	if err != nil {
		return nil, fmt.Errorf("failed to insert application: %w", err)
	}

	// 2. Insert Default Limits
	cpu := req.ResourceLimits.CPUQuota
	if cpu == "" {
		cpu = "100%"
	}
	ram := req.ResourceLimits.MemoryMax
	if ram == "" {
		ram = "512M"
	}
	tasks := req.ResourceLimits.TasksMax
	if tasks <= 0 {
		tasks = 128
	}

	_, err = tx.Exec(`
		INSERT INTO application_resource_limits (app_id, cpu_quota, memory_max, tasks_max)
		VALUES (?, ?, ?, ?)
	`, req.ID, cpu, ram, tasks)
	if err != nil {
		return nil, fmt.Errorf("failed to insert limits: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	req.ResourceLimits = contracts.ResourceLimits{
		CPUQuota:  cpu,
		MemoryMax: ram,
		TasksMax:  tasks,
	}

	return &req, nil
}

// ListApps retrieves applications visible to the user
func (s *AppService) ListApps(userID int64, isAdmin bool) ([]models.Application, error) {
	var query string
	var args []interface{}

	if isAdmin {
		query = `SELECT id, name, owner_id, team_id, server_id, runtime_type, runtime_version, git_repo, git_branch, port, build_command, start_command, health_path, assigned_unix_user, status, COALESCE(current_release, ''), created_at, updated_at FROM applications ORDER BY created_at DESC`
	} else {
		query = `SELECT id, name, owner_id, team_id, server_id, runtime_type, runtime_version, git_repo, git_branch, port, build_command, start_command, health_path, assigned_unix_user, status, COALESCE(current_release, ''), created_at, updated_at FROM applications WHERE owner_id = ? ORDER BY created_at DESC`
		args = append(args, userID)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []models.Application
	for rows.Next() {
		var a models.Application
		var teamID sql.NullInt64
		var buildCmd, healthPath sql.NullString

		if err := rows.Scan(
			&a.ID, &a.Name, &a.OwnerID, &teamID, &a.ServerID,
			&a.RuntimeType, &a.RuntimeVersion, &a.GitRepo, &a.GitBranch,
			&a.Port, &buildCmd, &a.StartCommand, &healthPath,
			&a.AssignedUnix, &a.Status, &a.CurrentRelease,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			continue
		}

		if teamID.Valid {
			a.TeamID = &teamID.Int64
		}
		if buildCmd.Valid {
			a.BuildCommand = buildCmd.String
		}
		if healthPath.Valid {
			a.HealthPath = healthPath.String
		}

		// Read limits
		_ = s.db.QueryRow(`SELECT cpu_quota, memory_max, tasks_max FROM application_resource_limits WHERE app_id = ?`, a.ID).Scan(
			&a.ResourceLimits.CPUQuota,
			&a.ResourceLimits.MemoryMax,
			&a.ResourceLimits.TasksMax,
		)

		// Read domains
		a.Domains = s.getAppDomains(a.ID)

		apps = append(apps, a)
	}

	return apps, nil
}

func (s *AppService) getAppDomains(appID string) []string {
	rows, err := s.db.Query(`SELECT domain FROM application_domains WHERE app_id = ?`, appID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err == nil {
			domains = append(domains, d)
		}
	}
	return domains
}

// GetApp returns single application and checks authorization
func (s *AppService) GetApp(appID string, userID int64, isAdmin bool) (*models.Application, error) {
	query := `SELECT id, name, owner_id, team_id, server_id, runtime_type, runtime_version, git_repo, git_branch, port, build_command, start_command, health_path, assigned_unix_user, status, COALESCE(current_release, ''), created_at, updated_at FROM applications WHERE id = ?`
	row := s.db.QueryRow(query, appID)

	var a models.Application
	var teamID sql.NullInt64
	var buildCmd, healthPath sql.NullString

	err := row.Scan(
		&a.ID, &a.Name, &a.OwnerID, &teamID, &a.ServerID,
		&a.RuntimeType, &a.RuntimeVersion, &a.GitRepo, &a.GitBranch,
		&a.Port, &buildCmd, &a.StartCommand, &healthPath,
		&a.AssignedUnix, &a.Status, &a.CurrentRelease,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAppNotFound
		}
		return nil, err
	}

	if !isAdmin && a.OwnerID != userID {
		return nil, ErrPermissionDenied
	}

	if teamID.Valid {
		a.TeamID = &teamID.Int64
	}
	if buildCmd.Valid {
		a.BuildCommand = buildCmd.String
	}
	if healthPath.Valid {
		a.HealthPath = healthPath.String
	}

	_ = s.db.QueryRow(`SELECT cpu_quota, memory_max, tasks_max FROM application_resource_limits WHERE app_id = ?`, a.ID).Scan(
		&a.ResourceLimits.CPUQuota,
		&a.ResourceLimits.MemoryMax,
		&a.ResourceLimits.TasksMax,
	)

	a.Domains = s.getAppDomains(a.ID)

	// Optionally sync status from agent
	if stat, err := s.agentClient.GetStatus(a.ID); err == nil {
		a.Status = stat.State
		_, _ = s.db.Exec(`UPDATE applications SET status = ? WHERE id = ?`, string(stat.State), a.ID)
	}

	return &a, nil
}

// UpdateApp updates application configuration
func (s *AppService) UpdateApp(appID string, userID int64, isAdmin bool, updates models.Application) error {
	app, err := s.GetApp(appID, userID, isAdmin)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		UPDATE applications SET 
			name = ?, git_repo = ?, git_branch = ?, port = ?,
			build_command = ?, start_command = ?, health_path = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, updates.Name, updates.GitRepo, updates.GitBranch, updates.Port,
		updates.BuildCommand, updates.StartCommand, updates.HealthPath, app.ID)
	if err != nil {
		return err
	}

	if updates.ResourceLimits.CPUQuota != "" {
		_, _ = s.db.Exec(`
			UPDATE application_resource_limits SET cpu_quota = ?, memory_max = ?, tasks_max = ? WHERE app_id = ?
		`, updates.ResourceLimits.CPUQuota, updates.ResourceLimits.MemoryMax, updates.ResourceLimits.TasksMax, app.ID)
	}

	return nil
}

// DeleteApp stops the application, removes reverse proxy, and cleans DB
func (s *AppService) DeleteApp(appID string, userID int64, isAdmin bool) error {
	app, err := s.GetApp(appID, userID, isAdmin)
	if err != nil {
		return err
	}

	// Stop service on agent
	_, _ = s.agentClient.ProcessAction(app.ID, "stop")

	// Delete from DB (foreign keys cascade)
	_, err = s.db.Exec(`DELETE FROM applications WHERE id = ?`, app.ID)
	return err
}

// ProcessAction starts, stops, or restarts the app service
func (s *AppService) ProcessAction(appID string, userID int64, isAdmin bool, action string) (*contracts.AgentProcessStatusResponse, error) {
	app, err := s.GetApp(appID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	resp, err := s.agentClient.ProcessAction(app.ID, action)
	if err != nil {
		return nil, err
	}

	_, _ = s.db.Exec(`UPDATE applications SET status = ? WHERE id = ?`, string(resp.State), app.ID)
	return resp, nil
}

// TriggerDeploy orchestrates release creation and calls host agent
func (s *AppService) TriggerDeploy(appID string, userID int64, isAdmin bool, triggeredBy string) (*models.Deployment, error) {
	app, err := s.GetApp(appID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	mu := s.getLock(app.ID)
	if !mu.TryLock() {
		return nil, ErrDeployInProgress
	}
	defer mu.Unlock()

	deployID := fmt.Sprintf("dep_%d", time.Now().UnixNano()%10000000)
	releaseID := fmt.Sprintf("%s_%s", time.Now().Format("20060102_150405"), deployID)

	// Create deployment record in DB
	_, err = s.db.Exec(`
		INSERT INTO deployments (id, app_id, commit_hash, commit_message, state, triggered_by)
		VALUES (?, ?, ?, ?, ?, ?)
	`, deployID, app.ID, "HEAD", "Deployment triggered via RunPilot", string(contracts.DeployStateBuilding), triggeredBy)
	if err != nil {
		return nil, err
	}

	// Read environment variables
	envVars := s.GetAppEnvVars(app.ID)

	// Send deploy instruction to agent
	agentReq := contracts.AgentDeployRequest{
		AppID:          app.ID,
		ReleaseID:      releaseID,
		RuntimeType:    app.RuntimeType,
		RuntimeVersion: app.RuntimeVersion,
		GitRepo:        app.GitRepo,
		GitBranch:      app.GitBranch,
		CommitHash:     "HEAD",
		Port:           app.Port,
		BuildCmd:       app.BuildCommand,
		StartCmd:       app.StartCommand,
		HealthPath:     app.HealthPath,
		EnvVars:        envVars,
		ResourceLimits: app.ResourceLimits,
	}

	agentResp, err := s.agentClient.Deploy(agentReq)
	finishTime := time.Now()

	deployment := &models.Deployment{
		ID:          deployID,
		AppID:       app.ID,
		CommitHash:  "HEAD",
		CommitMsg:   "Deployment triggered via RunPilot",
		TriggeredBy: triggeredBy,
		CreatedAt:   finishTime,
		FinishedAt:  &finishTime,
	}

	if err != nil || (agentResp != nil && !agentResp.Success) {
		errMsg := "deployment failed"
		if err != nil {
			errMsg = err.Error()
		} else if agentResp != nil && agentResp.ErrorMessage != "" {
			errMsg = agentResp.ErrorMessage
		}

		deployment.State = contracts.DeployStateFailed
		deployment.ErrorMessage = errMsg

		_, _ = s.db.Exec(`
			UPDATE deployments SET state = ?, error_message = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?
		`, string(contracts.DeployStateFailed), errMsg, deployID)

		// Record build log in deployment_logs
		if agentResp != nil && agentResp.BuildOutput != "" {
			_, _ = s.db.Exec(`INSERT INTO deployment_logs (deployment_id, log_line) VALUES (?, ?)`, deployID, agentResp.BuildOutput)
		}

		return deployment, fmt.Errorf("deployment failed: %s", errMsg)
	}

	deployment.State = contracts.DeployStateSuccessful
	_, _ = s.db.Exec(`
		UPDATE deployments SET state = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?
	`, string(contracts.DeployStateSuccessful), deployID)

	// Record build log
	if agentResp != nil && agentResp.BuildOutput != "" {
		_, _ = s.db.Exec(`INSERT INTO deployment_logs (deployment_id, log_line) VALUES (?, ?)`, deployID, agentResp.BuildOutput)
	}

	// Update app current_release and status
	_, _ = s.db.Exec(`
		UPDATE applications SET current_release = ?, status = ? WHERE id = ?
	`, releaseID, string(contracts.ProcessRunning), app.ID)

	return deployment, nil
}

// RollbackApp switches release pointer back to previous release
func (s *AppService) RollbackApp(appID string, userID int64, isAdmin bool, releaseID string) error {
	app, err := s.GetApp(appID, userID, isAdmin)
	if err != nil {
		return err
	}

	if err := s.agentClient.Rollback(app.ID, releaseID); err != nil {
		return err
	}

	_, _ = s.db.Exec(`UPDATE applications SET current_release = ?, status = ? WHERE id = ?`,
		releaseID, string(contracts.ProcessRunning), app.ID)

	return nil
}

// GetDeployments lists recent deployments for an application
func (s *AppService) GetDeployments(appID string) ([]models.Deployment, error) {
	rows, err := s.db.Query(`
		SELECT id, app_id, commit_hash, COALESCE(commit_message, ''), state, COALESCE(error_message, ''), triggered_by, duration_sec, created_at, finished_at
		FROM deployments WHERE app_id = ? ORDER BY created_at DESC LIMIT 20
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []models.Deployment
	for rows.Next() {
		var d models.Deployment
		if err := rows.Scan(&d.ID, &d.AppID, &d.CommitHash, &d.CommitMsg, &d.State, &d.ErrorMessage, &d.TriggeredBy, &d.DurationSec, &d.CreatedAt, &d.FinishedAt); err == nil {
			deps = append(deps, d)
		}
	}
	return deps, nil
}

// GetDeploymentLog retrieves stored build log for a deployment
func (s *AppService) GetDeploymentLog(deployID string) string {
	var logLine string
	_ = s.db.QueryRow(`SELECT log_line FROM deployment_logs WHERE deployment_id = ? ORDER BY id DESC LIMIT 1`, deployID).Scan(&logLine)
	return logLine
}

// GetAppEnvVars reads decrypted environment variables
func (s *AppService) GetAppEnvVars(appID string) map[string]string {
	rows, err := s.db.Query(`SELECT key_name, value_ciphertext FROM application_environment_variables WHERE app_id = ?`, appID)
	if err != nil {
		return map[string]string{}
	}
	defer rows.Close()

	masterKey := []byte(s.cfg.Server.MasterKey)
	res := make(map[string]string)

	for rows.Next() {
		var k, cipherText string
		if err := rows.Scan(&k, &cipherText); err == nil {
			dec, err := crypto.DecryptSecret(masterKey, cipherText)
			if err == nil {
				res[k] = dec
			} else {
				res[k] = cipherText
			}
		}
	}
	return res
}

// SetAppEnvVar writes encrypted environment variable
func (s *AppService) SetAppEnvVar(appID string, key, value string) error {
	masterKey := []byte(s.cfg.Server.MasterKey)
	cipherText, err := crypto.EncryptSecret(masterKey, value)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO application_environment_variables (app_id, key_name, value_ciphertext, is_secret)
		VALUES (?, ?, ?, 1)
		ON CONFLICT(app_id, key_name) DO UPDATE SET value_ciphertext = excluded.value_ciphertext
	`, appID, key, cipherText)
	return err
}

// AddDomain attaches a domain and configures Nginx reverse proxy
func (s *AppService) AddDomain(appID string, domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	_, err := s.db.Exec(`INSERT INTO application_domains (app_id, domain, ssl_enabled, ssl_status) VALUES (?, ?, 0, 'none')`, appID, domain)
	if err != nil {
		return err
	}

	// Trigger Nginx config reload on agent
	allDomains := s.getAppDomains(appID)
	app, _ := s.GetApp(appID, 0, true)
	if app != nil {
		_ = s.agentClient.ConfigureNginx(contracts.AgentNginxConfigRequest{
			AppID:            app.ID,
			Domains:          allDomains,
			UpstreamPort:     app.Port,
			SSLEnabled:       false,
			WebSocketEnabled: true,
		})
	}
	return nil
}

// RemoveDomain removes domain
func (s *AppService) RemoveDomain(appID string, domain string) error {
	_, err := s.db.Exec(`DELETE FROM application_domains WHERE app_id = ? AND domain = ?`, appID, domain)
	if err != nil {
		return err
	}

	allDomains := s.getAppDomains(appID)
	app, _ := s.GetApp(appID, 0, true)
	if app != nil {
		_ = s.agentClient.ConfigureNginx(contracts.AgentNginxConfigRequest{
			AppID:            app.ID,
			Domains:          allDomains,
			UpstreamPort:     app.Port,
			SSLEnabled:       false,
			WebSocketEnabled: true,
		})
	}
	return nil
}
