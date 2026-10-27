# RunPilot - Installation & Reinstallation Guide

Official Website: [https://runpilot.org](https://runpilot.org)  
Official Documentation: [https://runpilot.org/docs](https://runpilot.org/docs)  
GitHub Repository: [https://github.com/iamshahedul/runpilot](https://github.com/iamshahedul/runpilot)

RunPilot is an open-source, developer-first, standalone hosting control panel engineered specifically for **Node.js** and **Python** applications running as native Linux processes (`systemd` + `cgroups v2`).

---

## 1. Supported Operating Systems & Architectures

RunPilot runs on any standard Linux distribution with **systemd** and **cgroups v2**:

| Distribution | Supported Versions | Architecture | Init System |
|---|---|---|---|
| **Ubuntu** | 20.04 LTS, 22.04 LTS, 24.04 LTS | x86_64 / amd64, arm64 | systemd (required) |
| **Debian** | 11 (Bullseye), 12 (Bookworm) | x86_64 / amd64, arm64 | systemd (required) |
| **AlmaLinux / Rocky / RHEL** | 8.x, 9.x | x86_64 / amd64, arm64 | systemd (required) |

---

## 2. Server Architecture Invariants

- **100% Standalone Pure-Go Engine**: Zero dependency on external web servers like Nginx, Apache, or Bind9. RunPilot features its own built-in high-performance dynamic reverse proxy router listening directly on port 80 / 443.
- **Mandatory Zero-PHP & Zero-Composer Invariant**: The control plane, agent, and core daemons are strictly written in Go. RunPilot does not detect, require, or install PHP or Composer on your server.
- **Native Process Sandboxing**: Applications run as unprivileged Linux system users with dedicated systemd service units, protected by kernel namespaces and cgroups v2 resource quotas.
- **cPanel-Style Multi-Tenant Packages**: Built-in packages system (`Starter`, `Professional`, `Business`, `Custom`) and WHMCS module integration with per-user tenant isolation.

---

## 3. Hardware & Network Prerequisites

- **CPU**: 1 vCPU minimum (2+ vCPUs recommended).
- **RAM**: 1 GB RAM minimum (2 GB+ recommended for Node.js build pipelines and Python virtual environments).
- **Disk**: 5 GB available disk space for RunPilot binaries, application source codes, and SQLite database.
- **Network Ports**:
  - `80/tcp`: HTTP RunPilot Built-in Standalone Reverse Proxy Router & Dashboard
  - `443/tcp`: HTTPS TLS termination (Optional / Custom Domains)
  - `9090/tcp`: Host Agent internal privileged daemon API (binds to `127.0.0.1` locally)

---

## 4. Fresh Server Reinstallation Workflow (From GitHub)

If you have just rebuilt or reinstalled your VPS operating system (e.g., clean Ubuntu 22.04 / 24.04 LTS), follow this exact step-by-step workflow:

### Step 1: Install Base Dependencies & Go Compiler
Log in to your VPS as `root`:
```bash
sudo apt-get update && sudo apt-get install -y curl git tar openssl build-essential
```

Install modern Go (required for building binaries):
```bash
curl -fsSL https://go.dev/dl/go1.22.8.linux-amd64.tar.gz -o /tmp/go.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf /tmp/go.tar.gz
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
go version
```

### Step 2: Clone Repository from GitHub
```bash
cd /opt
sudo rm -rf /opt/runpilot-repo
sudo git clone https://github.com/iamshahedul/runpilot.git runpilot-repo
cd /opt/runpilot-repo
```

### Step 3: Run the Preflight Compatibility Check
Verify your server environment:
```bash
chmod +x installer/preflight.sh
./installer/preflight.sh
```

### Step 4: Run the Master Installer
Run the installer and specify your desired administrator email, password, and port:
```bash
chmod +x installer/install.sh
sudo ./installer/install.sh \
  --admin-email "admin@runpilot.org" \
  --admin-pass "YourStrongPassword2026!" \
  --port 80
```

### Step 5: Compile & Install RunPilot Standalone Binaries
Compile the control plane and host agent from source:
```bash
cd /opt/runpilot-repo
export PATH=$PATH:/usr/local/go/bin

# Build binaries
go build -o /opt/runpilot/bin/runpilot-server ./control-plane/cmd/runpilot-server
go build -o /opt/runpilot/bin/runpilot-agent ./agent/cmd/runpilot-agent

# Copy migrations, web dashboard, and systemd templates
sudo cp -r control-plane/migrations /opt/runpilot/
sudo cp -r control-plane/web /opt/runpilot/
sudo cp installer/templates/runpilot-server.service /etc/systemd/system/
sudo cp installer/templates/runpilot-agent.service /etc/systemd/system/

# Set file permissions
sudo chown -R runpilot:runpilot /opt/runpilot /srv/runpilot /var/log/runpilot /etc/runpilot
sudo chmod +x /opt/runpilot/bin/runpilot-server /opt/runpilot/bin/runpilot-agent
```

### Step 6: Enable and Start Systemd Daemons
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now runpilot-agent
sudo systemctl enable --now runpilot-server
```

Verify service status:
```bash
sudo systemctl status runpilot-server --no-pager
sudo systemctl status runpilot-agent --no-pager
```

### Step 7: Access Dashboard
Open your browser and navigate to:
```
http://<YOUR_VPS_IP>
```
Log in using either:
- **Username**: `admin` or your email: `admin@runpilot.org`
- **Password**: The password you specified during installation (or default `RunPilot2026!`).

---

## 5. Command-Line (CLI) User & Password Management

If you ever need to change your admin username, password, or email directly from the terminal without opening the web dashboard:

```bash
# Update password
/opt/runpilot/bin/runpilot-server -reset-user admin -new-pass "MyNewSecretPass!"

# Update email address
/opt/runpilot/bin/runpilot-server -reset-user admin -new-email "owner@runpilot.org"

# Update username
/opt/runpilot/bin/runpilot-server -reset-user admin -new-username "masteradmin"

# Change all credentials at once
/opt/runpilot/bin/runpilot-server -reset-user admin -new-username "superadmin" -new-email "contact@runpilot.org" -new-pass "Secure2026Pass!"
```

---

## 6. WHMCS Provisioning Module Setup

1. Copy `packages/whmcs-module/runpilot.php` into your WHMCS installation under:
   ```
   modules/servers/runpilot/runpilot.php
   ```
2. In WHMCS Admin:
   - Navigate to **System Settings > Products/Services > Servers**.
   - Click **Add New Server**, choose Module **RunPilot**, enter Server IP (`<YOUR_VPS_IP>`), Port `80`, Server Username (`admin`), and Server Password.
   - Click **Test Connection** (returns Success).
   - In **Products/Services**, edit your hosting product, select **Module: RunPilot**, and choose the **Hosting Package** (`Starter`, `Professional`, `Business`, or `Custom`).
3. Customer orders will automatically provision client accounts, assign package quotas, and display the cPanel-style credentials grid with 1-Click Auto Login (SSO).
