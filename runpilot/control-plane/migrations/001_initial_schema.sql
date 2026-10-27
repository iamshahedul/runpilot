-- ==============================================================================
-- RunPilot Control Plane - Initial Database Schema
-- Compatible with MySQL 8.x / MariaDB 10.x / SQLite 3.x
-- ==============================================================================

-- 1. Users
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(150) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user', -- 'admin', 'user', 'operator'
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. Teams
CREATE TABLE IF NOT EXISTS teams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL UNIQUE,
    slug VARCHAR(100) NOT NULL UNIQUE,
    owner_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 3. Team Members
CREATE TABLE IF NOT EXISTS team_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    team_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'member', -- 'owner', 'admin', 'member'
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (team_id, user_id),
    FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 4. Servers
CREATE TABLE IF NOT EXISTS servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL UNIQUE,
    hostname VARCHAR(255) NOT NULL,
    ip_address VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- 'active', 'offline', 'maintenance'
    os_info VARCHAR(150),
    cpu_cores INTEGER DEFAULT 1,
    memory_mb INTEGER DEFAULT 1024,
    disk_gb INTEGER DEFAULT 20,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 5. Server Agents
CREATE TABLE IF NOT EXISTS server_agents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL UNIQUE,
    endpoint VARCHAR(255) NOT NULL,
    auth_token_hash VARCHAR(255) NOT NULL,
    last_ping DATETIME,
    version VARCHAR(50),
    is_healthy INTEGER DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
);

-- 6. Applications
CREATE TABLE IF NOT EXISTS applications (
    id VARCHAR(64) PRIMARY KEY, -- e.g. "app_01j7..." or custom slug
    name VARCHAR(100) NOT NULL UNIQUE,
    owner_id INTEGER NOT NULL,
    team_id INTEGER,
    server_id INTEGER NOT NULL DEFAULT 1,
    runtime_type VARCHAR(50) NOT NULL, -- 'nodejs', 'python'
    runtime_version VARCHAR(50) NOT NULL, -- e.g. '20.x', '3.11'
    git_repo VARCHAR(500) NOT NULL,
    git_branch VARCHAR(100) NOT NULL DEFAULT 'main',
    port INTEGER NOT NULL DEFAULT 3000,
    build_command TEXT,
    start_command TEXT NOT NULL,
    health_path VARCHAR(255) DEFAULT '/',
    assigned_unix_user VARCHAR(64) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'stopped',
    current_release VARCHAR(64),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE SET NULL,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE RESTRICT
);

-- 7. Application Domains
CREATE TABLE IF NOT EXISTS application_domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id VARCHAR(64) NOT NULL,
    domain VARCHAR(255) NOT NULL UNIQUE,
    ssl_enabled INTEGER NOT NULL DEFAULT 0,
    ssl_status VARCHAR(50) NOT NULL DEFAULT 'none', -- 'none', 'pending', 'active', 'error'
    ssl_cert_path VARCHAR(500),
    ssl_key_path VARCHAR(500),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE
);

-- 8. Application Runtime Configs
CREATE TABLE IF NOT EXISTS application_runtime_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id VARCHAR(64) NOT NULL UNIQUE,
    node_package_manager VARCHAR(50) DEFAULT 'npm', -- 'npm', 'pnpm', 'yarn'
    python_venv_path VARCHAR(500),
    wsgi_asgi_runner VARCHAR(50) DEFAULT 'gunicorn', -- 'gunicorn', 'uvicorn'
    worker_processes INTEGER DEFAULT 1,
    timeout_seconds INTEGER DEFAULT 60,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE
);

-- 9. Application Environment Variables
CREATE TABLE IF NOT EXISTS application_environment_variables (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id VARCHAR(64) NOT NULL,
    key_name VARCHAR(150) NOT NULL,
    value_ciphertext TEXT NOT NULL, -- AES-256-GCM encrypted
    is_secret INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (app_id, key_name),
    FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE
);

-- 10. Encrypted Secrets (Deploy keys, server private keys, API credentials)
CREATE TABLE IF NOT EXISTS encrypted_secrets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scope VARCHAR(50) NOT NULL, -- 'app', 'server', 'system'
    reference_id VARCHAR(64) NOT NULL,
    secret_type VARCHAR(50) NOT NULL, -- 'git_deploy_key', 'api_key', 'tls_key'
    ciphertext TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 11. Deployments
CREATE TABLE IF NOT EXISTS deployments (
    id VARCHAR(64) PRIMARY KEY,
    app_id VARCHAR(64) NOT NULL,
    commit_hash VARCHAR(100) NOT NULL DEFAULT 'HEAD',
    commit_message TEXT,
    state VARCHAR(50) NOT NULL DEFAULT 'queued', -- 'queued','fetching','building','configuring','starting','verifying','successful','failed','rolled_back'
    error_message TEXT,
    triggered_by VARCHAR(100) NOT NULL,
    duration_sec INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE
);

-- 12. Deployment Steps
CREATE TABLE IF NOT EXISTS deployment_steps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id VARCHAR(64) NOT NULL,
    step_name VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    duration_ms INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (deployment_id) REFERENCES deployments(id) ON DELETE CASCADE
);

-- 13. Deployment Logs (Bounded storage)
CREATE TABLE IF NOT EXISTS deployment_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id VARCHAR(64) NOT NULL,
    log_line TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (deployment_id) REFERENCES deployments(id) ON DELETE CASCADE
);

-- 14. Application Processes
CREATE TABLE IF NOT EXISTS application_processes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id VARCHAR(64) NOT NULL UNIQUE,
    pid INTEGER DEFAULT 0,
    systemd_unit VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'stopped',
    memory_bytes INTEGER DEFAULT 0,
    cpu_percent REAL DEFAULT 0.0,
    uptime_sec INTEGER DEFAULT 0,
    last_exit_code INTEGER DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE
);

-- 15. Application Resource Limits (cgroups v2)
CREATE TABLE IF NOT EXISTS application_resource_limits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id VARCHAR(64) NOT NULL UNIQUE,
    cpu_quota VARCHAR(50) DEFAULT '100%',
    memory_max VARCHAR(50) DEFAULT '512M',
    tasks_max INTEGER DEFAULT 128,
    disk_quota VARCHAR(50) DEFAULT '5G',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE
);

-- 16. Background Workers
CREATE TABLE IF NOT EXISTS background_workers (
    id VARCHAR(64) PRIMARY KEY,
    app_id VARCHAR(64) NOT NULL,
    name VARCHAR(100) NOT NULL,
    command TEXT NOT NULL,
    instances INTEGER DEFAULT 1,
    status VARCHAR(50) DEFAULT 'stopped',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE
);

-- 17. Scheduled Jobs
CREATE TABLE IF NOT EXISTS scheduled_jobs (
    id VARCHAR(64) PRIMARY KEY,
    app_id VARCHAR(64) NOT NULL,
    name VARCHAR(100) NOT NULL,
    schedule VARCHAR(100) NOT NULL, -- e.g. '0 * * * *'
    command TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    last_run DATETIME,
    last_status VARCHAR(50),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE
);

-- 18. Backups
CREATE TABLE IF NOT EXISTS backups (
    id VARCHAR(64) PRIMARY KEY,
    app_id VARCHAR(64) NOT NULL,
    backup_type VARCHAR(50) NOT NULL DEFAULT 'full',
    file_path VARCHAR(500) NOT NULL,
    size_bytes INTEGER DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'completed',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE
);

-- 19. API Tokens
CREATE TABLE IF NOT EXISTS api_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name VARCHAR(100) NOT NULL,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    last_used_at DATETIME,
    expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 20. Audit Events
CREATE TABLE IF NOT EXISTS audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    app_id VARCHAR(64),
    action VARCHAR(100) NOT NULL,
    ip_address VARCHAR(100),
    details TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 21. System Settings
CREATE TABLE IF NOT EXISTS system_settings (
    key_name VARCHAR(100) PRIMARY KEY,
    value_text TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed default initial system record
INSERT OR IGNORE INTO system_settings (key_name, value_text) VALUES 
('installed_version', '1.0.0'),
('allow_registration', 'true'),
('default_node_version', '20.x'),
('default_python_version', '3.11');

-- Seed default local server
INSERT OR IGNORE INTO servers (id, name, hostname, ip_address, status, os_info) VALUES
(1, 'Primary Node', 'localhost', '127.0.0.1', 'active', 'Linux (Native systemd host)');
