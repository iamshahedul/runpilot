# RunPilot

> **Run applications. Own your stack.**

RunPilot is an open-source, developer-first application hosting control panel engineered specifically for **Node.js** and **Python** applications running as native Linux processes.

Unlike traditional control panels built around legacy stacks, RunPilot is built from the ground up with **Go**, **systemd**, **cgroups v2**, and **Nginx** to provide modern, lightweight, developer-friendly process isolation without requiring Docker.

---

## Key Highlights

- **Native Linux Processes**: High performance native processes supervised by `systemd`. No Docker overhead.
- **Node.js Runtime Management**: Support for multiple Node.js versions (18.x, 20.x, 22.x), package managers (`npm`, `pnpm`, `yarn`), Express, Fastify, NestJS, and Next.js.
- **Python Runtime Management**: Dedicated virtual environments (`venv`), `requirements.txt` / `pyproject.toml`, WSGI/ASGI application runners (`Gunicorn`, `Uvicorn`), Django, Flask, and FastAPI.
- **Git Deployment Engine**: Zero-downtime release deployments, atomic symlink switching (`current` -> `releases/<id>`), and instant rollbacks.
- **Reverse Proxy & TLS**: Automated Nginx virtual host generation, WebSocket upgrade proxying, and ACME SSL certificates.
- **Strict Multi-Tenant Isolation**: Unprivileged dedicated system users (`runpilot-app-<id>`), kernel sandboxing (`ProtectSystem=full`), and cgroups v2 resource limits (`CPUQuota`, `MemoryMax`, `TasksMax`).
- **Zero PHP / Composer Footprint**: Absolutely NO PHP or Composer detection, execution, or dependencies anywhere in installation or runtime.

---

## Architecture

RunPilot is organized into three core components:

1. **Control Plane (`control-plane/`)**: Modern Go REST API and sleek dark dashboard (`#0B1120`) managing metadata, RBAC, tenant permissions, and deployment orchestration.
2. **Host Agent (`agent/`)**: Privileged Go daemon on managed nodes executing validated, non-arbitrary operations (user creation, systemd unit generation, log streaming, Nginx configuration).
3. **Application Runtime**: Unprivileged native Linux processes running under dedicated service units.

---

## Quickstart

### 1. Preflight System Checker
Verify your host system before running the installer:
```bash
./installer/preflight.sh
```

### 2. Run Installer
```bash
sudo ./installer/install.sh
```

### 3. Build From Source
```bash
# Build Control Plane, Agent, and CLI
make build

# Run Automated Test Suite
make test
```

---

## Documentation

- [Installation & Prerequisites](docs/installation.md)
- [Architecture & Design](docs/architecture.md)
- [Security Model](docs/security.md)
- [REST API Specification](docs/api.md)
- [Troubleshooting](docs/troubleshooting.md)

---

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
