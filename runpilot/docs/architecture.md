# RunPilot - Architecture & Component Design

Official Website: [https://runpilot.org](https://runpilot.org)  
Official Documentation: [https://runpilot.org/docs](https://runpilot.org/docs)

RunPilot separates responsibilities into three principal components designed for security, native Linux process performance, and auditability.

```
+-----------------------------------------------------------------------------------+
|                            RUNPILOT ARCHITECTURE                                  |
+-----------------------------------------------------------------------------------+
|                                                                                   |
|  [ Web Browser / Developer CLI (runpilot) / WHMCS Billing Module ]                |
|                         │                                                         |
|                         ▼ (HTTP :80 / :443)                                       |
|  +─────────────────────────────────────────────────────────────+                  |
|  |             COMPONENT A: CONTROL PLANE & NATIVE ROUTER (Go) |                  |
|  |  - Built-in Dynamic Host Router (Zero Nginx Dependency)     |                  |
|  |  - Web UI Dashboard (#0B1120 Sleek Dark Mobile Responsive)  |                  |
|  |  - Auth & Role-Based Access Control (Admin / Tenant / User) |                  |
|  |  - Hosting Packages Engine (Starter, Pro, Business, Custom) |                  |
|  |  - Metadata Store (Pure Go SQLite Engine)                   |                  |
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
|  |   Upstream Proxy -> 127.0.0.1:3000                          |                  |
|  +─────────────────────────────────────────────────────────────+                  |
|                                                                                   |
+-----------------------------------------------------------------------------------+
```

---

## 1. Component A: Control Plane & Dynamic Router

The Control Plane exposes the REST API, web dashboard, and built-in dynamic proxy:
- **Native Host Routing**: Pure-Go dynamic reverse proxy listening on port 80/443. Traffic accessing via IP or panel domains routes to dashboard & API; traffic accessing via client domains maps instantly to internal app ports without external web server reloads.
- **Authentication**: Modern password hashing with bcrypt (cost 12), HTTP-only secure cookies, and bearer JWT tokens for API / CLI / WHMCS SSO.
- **Tenant Isolation**: Every application belongs to an owner or client. Non-admin users only access applications where `owner_id = user.ID`.
- **Hosting Packages**: Packages define CPUQuota, MemoryMax, TasksMax, DiskQuota, MaxApps, and MaxDomains.
- **Database Schema**: Pure Go SQLite storage engine (`modernc.org/sqlite`), zero CGO compilation required.

---

## 2. Component B: Privileged Host Agent

The Host Agent runs as root with minimal attack surface:
- **Sandbox Management**: Provisions locked system users (`runpilot-app-<id>`) with no interactive login shell (`/usr/sbin/nologin`).
- **Process Supervision**: Directly interacts with Linux systemd (`PID 1`) over DBus / unit generation.
- **cgroups v2 Resource Governance**: Enforces hard resource caps at kernel level.
- **Journal Log Streaming**: Reads process stdout and stderr buffers directly from systemd journald.

---

## 3. Component C: Isolated Application Runtimes

- **Node.js Environment**: Standard Node.js versions (18.x, 20.x, 22.x) running natively.
- **Python Environment**: Isolated virtual environments (`venv`) with WSGI/ASGI servers (`Gunicorn`, `Uvicorn`).
- **Directory Structure**:
  ```
  /srv/runpilot/apps/<app-id>/
  ├── releases/
  │   └── <release-id>/      # Immutable release code
  ├── current -> releases/<id>  # Atomic symlink
  └── shared/
      ├── .env              # AES-256 decrypted environment (0600)
      └── logs/             # Persistent logs
  ```
