# RunPilot - Architecture & Component Design

RunPilot separates responsibilities into three principal components designed for security, native Linux process performance, and auditability.

```
+-----------------------------------------------------------------------------------+
|                            RUNPILOT ARCHITECTURE                                  |
+-----------------------------------------------------------------------------------+
|                                                                                   |
|  [ Web Browser / Developer CLI (runpilot) ]                                       |
|                         │                                                         |
|                         ▼ (REST API / JWT Auth)                                   |
|  +─────────────────────────────────────────────────────────────+                  |
|  |             COMPONENT A: CONTROL PLANE (Go)                 |                  |
|  |  - Web UI Dashboard (#0B1120 Sleek Dark Theme)              |                  |
|  |  - Auth & Role-Based Access Control (Admin / Tenant / User) |                  |
|  |  - Application Metadata Store (MySQL / MariaDB / SQLite)    |                  |
|  |  - Deployment Job Queue & Concurrency Orchestrator          |                  |
|  |  - Audit Log Event Stream                                   |                  |
|  +──────────────────────────────┬──────────────────────────────+                  |
|                                 │                                                 |
|                                 ▼ (Secure Internal API / HMAC Token Auth)          |
|  +─────────────────────────────────────────────────────────────+                  |
|  |              COMPONENT B: HOST AGENT (Go)                   |                  |
|  |  - Dedicated Privileged Daemon (systemd supervisor)         |                  |
|  |  - Provisions unprivileged Unix users (runpilot-app-<id>)   |                  |
|  |  - Git clone / checkout in safe workspace                   |                  |
|  |  - Node.js & Python isolated runtime execution              |                  |
|  |  - Generates deterministic systemd service units            |                  |
|  |  - Enforces cgroups v2 limits (CPUQuota, MemoryMax)         |                  |
|  |  - Validates and writes Nginx reverse proxy configs         |                  |
|  |  - Tails process stdout/stderr journal logs                 |                  |
|  +──────────────────────────────┬──────────────────────────────+                  |
|                                 │                                                 |
|                                 ▼ (Native Linux Processes)                        |
|  +─────────────────────────────────────────────────────────────+                  |
|  |          COMPONENT C: APPLICATION RUNTIMES                  |                  |
|  |                                                             |                  |
|  |   Node.js Application              Python Application       |                  |
|  |   UID: runpilot-app-101            UID: runpilot-app-102    |                  |
|  |   Runtime: Node 18/20/22           Runtime: Python 3.10/3.12|                  |
|  |   Engine: npm / pnpm / yarn        Engine: venv / gunicorn  |                  |
|  |   systemd: runpilot-app-101.service                         |                  |
|  |   Limits: CPU 100%, RAM 512MB                               |                  |
|  |   Nginx upstream -> 127.0.0.1:3000                          |                  |
|  +─────────────────────────────────────────────────────────────+                  |
|                                                                                   |
+-----------------------------------------------------------------------------------+
```

---

## 1. Component A: Control Plane

The Control Plane exposes the REST API and the web dashboard:
- **Authentication**: Modern password hashing with Argon2id / bcrypt, HTTP-only secure cookies, and bearer JWT tokens for API / CLI interactions.
- **Tenant Isolation**: Every application belongs to an owner or team. Tenant ownership is enforced at the API database query layer and passed down to the agent.
- **Job Orchestration**: Asynchronous deployment state transitions:
  `queued` -> `fetching` -> `building` -> `configuring` -> `starting` -> `verifying` -> `successful` (or `failed` / `rolled_back`).
- **Database Schema**: MySQL / MariaDB compatible, with SQLite fallback support for single-node installations and automated tests.

---

## 2. Component B: Host Agent

The Host Agent runs as a privileged daemon on the managed host:
- **Restricted API**: The agent never exposes an arbitrary root shell or unvalidated bash script execution endpoint.
- **Safe Operations**: Only pre-compiled, strictly typed, parameter-validated actions are permitted (e.g., `DeployApp`, `StartService`, `StopService`, `RestartService`, `GetAppStatus`, `ReadAppLogs`, `ApplyNginxConfig`).
- **Security Perimeter**: All communication with the Control Plane requires a shared secret token validated on each request.

---

## 3. Component C: Application Runtime & Directory Structure

Applications run strictly as unprivileged Unix identities (`runpilot-app-<id>`). They never execute as `root` or `runpilot`.

The deployment filesystem follows an atomic symlink release layout:

```
/srv/runpilot/apps/<app_id>/
├── releases/
│   ├── 20261027_200001/         # Pinned release directory
│   │   ├── package.json / requirements.txt
│   │   ├── app code...
│   │   └── node_modules / .venv (or shared cache)
│   └── 20261027_193000/         # Previous release (for instant rollback)
├── shared/
│   ├── .env                     # Secure application environment variables
│   ├── logs/                    # Application stdout/stderr logs
│   └── uploads/                 # Persistent storage preserved across releases
└── current -> releases/20261027_200001  # Atomic symlink switched after health check
```

---

## 4. systemd Service Unit Blueprint

Deterministic unit name: `runpilot-app-<id>.service`

Example generated template:
```ini
[Unit]
Description=RunPilot Application AppID: %i
After=network.target

[Service]
Type=simple
User=runpilot-app-%i
Group=runpilot-app-%i
WorkingDirectory=/srv/runpilot/apps/%i/current
EnvironmentFile=/srv/runpilot/apps/%i/shared/.env
ExecStart=/srv/runpilot/runtimes/nodejs/current/bin/node server.js
Restart=on-failure
RestartSec=5s

# Sandboxing & Hardening
ProtectSystem=full
ProtectHome=true
NoNewPrivileges=true
PrivateTmp=true

# Resource Limits (cgroups v2)
CPUQuota=100%
MemoryMax=512M
TasksMax=128

[Install]
WantedBy=multi-user.target
```

---

## 5. Nginx Reverse Proxy Architecture

Applications bind to localhost ports (or Unix domain sockets). RunPilot validates upstream routes and writes atomic configuration files into `/etc/nginx/conf.d/runpilot-app-<id>.conf`:
- HTTP-to-HTTPS automatic 301 redirection
- WebSocket upgrade headers (`Upgrade $http_upgrade`, `Connection "upgrade"`)
- Timeouts and buffer configurations
- ACME challenge path routing (`/.well-known/acme-challenge/`)
