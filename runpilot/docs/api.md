# RunPilot - REST API Specification

The RunPilot REST API provides programmatic control over applications, runtimes, deployments, domains, and server agents.

All requests accept and return `application/json` unless otherwise specified.

---

## 1. Authentication Endpoints

### `POST /api/v1/auth/login`
Authenticate user with username/email and password.

**Request Body:**
```json
{
  "email": "admin@example.com",
  "password": "StrongPassword123!"
}
```

**Response (200 OK):**
```json
{
  "token": "eyJhbGciOi...",
  "user": {
    "id": 1,
    "email": "admin@example.com",
    "name": "Platform Administrator",
    "role": "admin"
  }
}
```

### `GET /api/v1/auth/me`
Retrieve currently authenticated user profile and permissions.

---

## 2. Server & Agent Health Endpoints

### `GET /api/v1/servers`
List registered servers and connection status.

### `GET /api/v1/servers/{id}/stats`
Retrieve live CPU, memory, disk, and load metrics from host agent.

**Response (200 OK):**
```json
{
  "server_id": 1,
  "hostname": "prod-node-01",
  "os": "Ubuntu 22.04 LTS",
  "cpu_percent": 14.2,
  "memory": {
    "total_mb": 8192,
    "used_mb": 2048,
    "free_mb": 6144
  },
  "disk": {
    "total_gb": 120,
    "used_gb": 22,
    "free_gb": 98
  },
  "active_apps": 5
}
```

---

## 3. Application Lifecycle Endpoints

### `GET /api/v1/applications`
List all applications (filtered by tenant ownership for non-admin users).

### `POST /api/v1/applications`
Register a new application.

**Request Body:**
```json
{
  "name": "my-express-api",
  "runtime_type": "nodejs",
  "runtime_version": "20.x",
  "git_repo": "https://github.com/example/api.git",
  "git_branch": "main",
  "port": 3000,
  "build_command": "npm install && npm run build",
  "start_command": "node dist/index.js",
  "env_vars": {
    "PORT": "3000",
    "NODE_ENV": "production"
  },
  "cpu_limit": "100%",
  "memory_limit": "512M"
}
```

### `GET /api/v1/applications/{id}`
Get application details, status, and configuration.

### `PUT /api/v1/applications/{id}`
Update application configuration.

### `DELETE /api/v1/applications/{id}`
Gracefully stop systemd service, remove Nginx site config, delete Unix user, and archive application files.

---

## 4. Application Process Control

### `POST /api/v1/applications/{id}/start`
Start application systemd unit.

### `POST /api/v1/applications/{id}/stop`
Stop application systemd unit.

### `POST /api/v1/applications/{id}/restart`
Perform graceful restart of application systemd unit.

### `GET /api/v1/applications/{id}/logs`
Read recent journal logs from application process.

---

## 5. Deployment Engine

### `POST /api/v1/applications/{id}/deploy`
Trigger new Git deployment.

**Request Body:**
```json
{
  "commit": "HEAD",
  "message": "Manual deployment via dashboard"
}
```

### `GET /api/v1/applications/{id}/deployments`
List deployment history with timestamps and status.

### `POST /api/v1/applications/{id}/rollback`
Rollback `current` symlink to a previous successful release.

---

## 6. Domains & Reverse Proxy

### `POST /api/v1/applications/{id}/domains`
Attach a custom domain to an application.

### `DELETE /api/v1/applications/{id}/domains/{domain}`
Detach domain and re-validate Nginx reverse proxy routing.
