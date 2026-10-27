# RunPilot - REST API Specification

Official Website: [https://runpilot.org](https://runpilot.org)  
Official Documentation: [https://runpilot.org/docs](https://runpilot.org/docs)

The RunPilot REST API provides programmatic control over applications, hosting packages, runtimes, deployments, domains, users, and server agents.

All requests accept and return `application/json` with standard HTTP status codes.

---

## 1. Authentication Endpoints

### `POST /api/v1/auth/login`
Authenticate user with username or email and password.

**Request Body:**
```json
{
  "identifier": "admin",
  "password": "StrongPassword123!"
}
```

**Response (200 OK):**
```json
{
  "token": "eyJhbGciOi...",
  "user": {
    "id": 1,
    "username": "admin",
    "email": "admin@runpilot.org",
    "full_name": "Platform Administrator",
    "role": "admin"
  }
}
```

### `GET /api/v1/auth/me`
Retrieve currently authenticated user profile and active claims.

### `POST /api/v1/auth/sso-token`
Generate a scoped Single Sign-On (SSO) token for a specific client user (used by WHMCS 1-Click Login).

---

## 2. Profile & Account Management

### `GET /api/v1/profile`
Retrieve the authenticated user's profile details.

### `PUT /api/v1/profile`
Update username, email address, and full name.

**Request Body:**
```json
{
  "username": "superadmin",
  "email": "contact@runpilot.org",
  "full_name": "Chief Administrator"
}
```

### `PUT /api/v1/profile/password`
Change authenticated user's password with old password verification.

**Request Body:**
```json
{
  "old_password": "CurrentPassword123!",
  "new_password": "NewSecretPassword2026!"
}
```

---

## 3. Hosting Packages Endpoints (cPanel-Style Plans)

### `GET /api/v1/packages`
List all configured hosting packages.

### `POST /api/v1/packages` (Admin Only)
Create a new hosting package plan.

**Request Body:**
```json
{
  "name": "Enterprise",
  "slug": "enterprise",
  "memory_max": "4G",
  "cpu_quota": "200%",
  "disk_quota": "50G",
  "tasks_max": 512,
  "max_apps": 10,
  "max_domains": 20
}
```

---

## 4. User & Client Management Endpoints

### `GET /api/v1/users` (Admin Only)
List all platform users and client hosting accounts.

### `POST /api/v1/users` (Admin Only)
Provision or update a client user account.

**Request Body:**
```json
{
  "username": "clientuser",
  "email": "client@example.com",
  "password": "ClientPassword123!",
  "full_name": "John Doe",
  "role": "user"
}
```

### `POST /api/v1/users/reset-password` (Admin Only)
Directly reset any user's password by username or email.

---

## 5. Application Lifecycle Endpoints

### `GET /api/v1/applications`
List all applications (filtered automatically by tenant ownership for client users).

### `POST /api/v1/applications`
Create and register a new application.

**Request Body:**
```json
{
  "name": "My Express API",
  "package": "Starter",
  "runtime_type": "nodejs",
  "runtime_version": "20.x",
  "git_repo": "https://github.com/example/api.git",
  "git_branch": "main",
  "port": 3000,
  "start_command": "node server.js",
  "resource_limits": {
    "cpu_quota": "100%",
    "memory_max": "512M"
  }
}
```

### `POST /api/v1/applications/{id}/deploy`
Trigger deployment and release build.

### `POST /api/v1/applications/{id}/restart`
Restart the application's native systemd service unit.

### `POST /api/v1/applications/{id}/stop`
Stop application process (e.g. on invoice suspension).

### `POST /api/v1/applications/{id}/start`
Start application process.

### `GET /api/v1/applications/{id}/logs?lines=100`
Stream recent systemd journal logs.
