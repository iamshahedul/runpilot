# RunPilot - Installation & Operating System Support

RunPilot is an open-source, developer-first hosting control panel engineered specifically for **Node.js** and **Python** applications running as native Linux processes.

---

## 1. Supported Operating Systems & Architectures

RunPilot officially supports modern Linux distributions that utilize **systemd** as the init system and support **cgroups v2**:

| Distribution | Supported Versions | Architecture | Init System |
|---|---|---|---|
| **Ubuntu** | 20.04 LTS, 22.04 LTS, 24.04 LTS | x86_64 / amd64, aarch64 / arm64 | systemd (required) |
| **Debian** | 11 (Bullseye), 12 (Bookworm) | x86_64 / amd64, aarch64 / arm64 | systemd (required) |
| **AlmaLinux** | 8.x, 9.x | x86_64 / amd64, aarch64 / arm64 | systemd (required) |
| **Rocky Linux** | 8.x, 9.x | x86_64 / amd64, aarch64 / arm64 | systemd (required) |
| **RHEL** | 8.x, 9.x | x86_64 / amd64, aarch64 / arm64 | systemd (required) |

> **Local Development Note**: macOS (Darwin) is supported for developing, testing, and running the control-plane REST API, Web dashboard, and CLI in development/simulation mode. Native process management with systemd units and cgroups requires a Linux host.

---

## 2. Hardware Prerequisites

- **CPU**: 1 core minimum (2+ cores recommended for multi-tenant application builds).
- **RAM**: 1 GB RAM minimum (2 GB+ recommended for running Node.js build pipelines & Python wheels).
- **Disk**: 5 GB available disk space for RunPilot binaries, runtime caches, release directories, and database storage.
- **Network Ports**:
  - `80/tcp`: HTTP reverse proxy (Nginx) & ACME challenge validation
  - `443/tcp`: HTTPS reverse proxy (Nginx) with TLS termination
  - `8080/tcp`: RunPilot Control Plane Web UI & REST API
  - `9090/tcp`: RunPilot Host Agent internal privileged daemon API (binds to `127.0.0.1` on single-server installs)

---

## 3. Mandatory Zero-PHP Policy

RunPilot is **100% PHP-free and Composer-free**:
- RunPilot **does not require**, **does not detect**, and **does not install** PHP or Composer.
- The control plane, host agent, and CLI are written in compiled **Go**.
- Application runtimes supported are natively **Node.js** (`npm`, `pnpm`, `yarn`) and **Python** (`venv`, `pip`, `gunicorn`, `uvicorn`).
- A clean Linux installation without PHP or Composer will install and operate with zero warnings or errors.

---

## 4. Required Native System Packages

The installer expects standard Linux utilities:
- `curl`, `tar`, `openssl` (for TLS generation & secure tokens)
- `git` (for Git-based repository deployment)
- `nginx` (for reverse-proxy routing and virtual host management)
- `systemd` (PID 1 process lifecycle supervisor)
- `cgroups v2` (for CPU, memory, and task isolation limits)

On Debian/Ubuntu:
```bash
sudo apt-get update
sudo apt-get install -y curl tar git openssl nginx
```

On RHEL/AlmaLinux/Rocky:
```bash
sudo dnf install -y curl tar git openssl nginx
```

---

## 5. Quick Installation Workflow

### Step 1: Run Preflight Check
Before installation, verify your server prerequisites without altering the system:

```bash
chmod +x installer/preflight.sh
./installer/preflight.sh
```

For automated provisioning pipelines, pass `--json`:
```bash
./installer/preflight.sh --json
```

### Step 2: Run Master Installer
Install the RunPilot control plane, host agent, configuration directories, and service accounts:

```bash
chmod +x installer/install.sh
sudo ./installer/install.sh
```

Options:
- `--install-dir <path>`: Base installation path (default: `/opt/runpilot`)
- `--data-dir <path>`: Application deployment root (default: `/srv/runpilot`)
- `--port <port>`: Control plane port (default: `8080`)
- `--agent-port <port>`: Agent API port (default: `9090`)
- `--dry-run`: Preview actions without touching system state

### Step 3: Access Dashboard
Once services are started, access the Web Dashboard at:
```
http://<server-ip>:8080
```
Login with the administrative credentials generated during installation.
