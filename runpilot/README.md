# RunPilot

> **Native Node.js & Python Application Hosting. Own your stack.**

Official Website: [https://runpilot.org](https://runpilot.org)  
Documentation: [https://runpilot.org/docs](https://runpilot.org/docs)  
GitHub Repository: [https://github.com/iamshahedul/runpilot](https://github.com/iamshahedul/runpilot)

RunPilot is an open-source, developer-first application hosting control panel engineered specifically for **Node.js** and **Python** applications running as native Linux processes.

Unlike traditional control panels built around heavy legacy stacks or complex Docker overlays, RunPilot is built from the ground up in compiled **Go**, leveraging native Linux kernel features: **systemd**, **cgroups v2**, and a built-in high-performance **Dynamic Host Router**.

---

## Key Highlights

- **100% Standalone Pure-Go Engine**: Zero dependency on external web servers (Nginx, Apache, or Bind9). Runs directly on port 80/443 with its own native dynamic reverse proxy router.
- **Native Linux Processes**: High performance native processes supervised by `systemd`. No container or Docker virtualization overhead.
- **Node.js Runtime Management**: Support for multiple Node.js versions (18.x, 20.x, 22.x), package managers (`npm`, `pnpm`, `yarn`), Express, Fastify, NestJS, and Next.js.
- **Python Runtime Management**: Dedicated virtual environments (`venv`), WSGI/ASGI application runners (`Gunicorn`, `Uvicorn`), Django, Flask, and FastAPI.
- **cPanel-Style Hosting Packages**: Resource quota management (`Starter`, `Professional`, `Business`, `Custom`) with RAM, CPU, disk, tasks, and domain limits.
- **Multi-Tenant Isolation**: Every client service gets its own tenant username, email, and password. Non-admin users only see and manage their own applications.
- **100% Mobile Responsive Dashboard**: Sleek dark UI (`#0B1120`), slide-out mobile drawer, touch-friendly tables, and responsive forms.
- **WHMCS Provisioning Module**: Complete automation with Test Connection, Create Account, Suspend, Unsuspend, Terminate, Package Change, and 1-Click SSO login.
- **Zero PHP / Composer Footprint**: Strict architectural invariant verified by automated gate tests — absolutely 0 PHP or Composer checks, installations, or dependencies.

---

## Architecture Overview

```
                          [ Internet Traffic ]
                                   │
                                   ▼
         ┌───────────────────────────────────────────────────┐
         │       RunPilot Native Dynamic Router (:80/:443)    │
         └─────────┬───────────────────────────────┬─────────┘
                   │                               │
        (IP / Localhost / API)            (Client Custom Domains)
                   │                               │
                   ▼                               ▼
      ┌─────────────────────────┐     ┌─────────────────────────┐
      │ RunPilot Control Plane  │     │ Native App Processes    │
      │ • REST API & Web UI     │     │ • Node.js (Express/Next)│
      │ • SQLite Database       │     │ • Python (FastAPI/Flask)│
      │ • JWT & Multi-Tenant    │     │ • Sandboxed (cgroups v2)│
      └─────────────────────────┘     └─────────────────────────┘
```

---

## Quickstart & Fresh Installation (Ubuntu/Debian)

### 1. Clone Repository
```bash
sudo git clone https://github.com/iamshahedul/runpilot.git /opt/runpilot-repo
cd /opt/runpilot-repo
```

### 2. Run Preflight Check
Verify system compatibility:
```bash
chmod +x installer/preflight.sh
./installer/preflight.sh
```

### 3. Run Master Installer
Install with custom administrator credentials:
```bash
chmod +x installer/install.sh
sudo ./installer/install.sh \
  --admin-email "admin@runpilot.org" \
  --admin-pass "YourSecurePassword2026!" \
  --port 80
```

### 4. Build and Start Services
```bash
go build -o /opt/runpilot/bin/runpilot-server ./control-plane/cmd/runpilot-server
go build -o /opt/runpilot/bin/runpilot-agent ./agent/cmd/runpilot-agent

sudo cp -r control-plane/migrations /opt/runpilot/
sudo cp -r control-plane/web /opt/runpilot/
sudo cp installer/templates/runpilot-server.service /etc/systemd/system/
sudo cp installer/templates/runpilot-agent.service /etc/systemd/system/

sudo systemctl daemon-reload
sudo systemctl enable --now runpilot-agent
sudo systemctl enable --now runpilot-server
```

---

## CLI Credentials Management

Update username, email, or password directly from the command line:

```bash
# Change password
/opt/runpilot/bin/runpilot-server -reset-user admin -new-pass "NewPassword!"

# Change email address
/opt/runpilot/bin/runpilot-server -reset-user admin -new-email "contact@runpilot.org"

# Change username
/opt/runpilot/bin/runpilot-server -reset-user admin -new-username "superadmin"
```

---

## Documentation

- [Installation & Reinstallation Guide](docs/installation.md)
- [Architecture & Design](docs/architecture.md)
- [Security Model](docs/security.md)
- [REST API Specification](docs/api.md)
- [Troubleshooting](docs/troubleshooting.md)

---

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
