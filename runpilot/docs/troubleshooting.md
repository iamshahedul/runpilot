# RunPilot - Troubleshooting & Operational Diagnostics

Official Website: [https://runpilot.org](https://runpilot.org)  
Official Documentation: [https://runpilot.org/docs](https://runpilot.org/docs)

This guide provides step-by-step diagnostic procedures for common operational situations.

---

## 1. Port 80 Binding & Standalone Router Diagnostics

RunPilot runs as a 100% standalone binary that binds directly to port 80 (and 443) with its own high-performance dynamic reverse proxy router.

### A. Port 80 Conflict (e.g. Apache or default Nginx already running)
If `runpilot-server` fails to start because port 80 is occupied:
```bash
sudo ss -tulpn | grep :80
```
If an old Apache or Nginx process is running, disable and stop it:
```bash
sudo systemctl stop apache2 2>/dev/null || true
sudo systemctl disable apache2 2>/dev/null || true
sudo systemctl stop nginx 2>/dev/null || true
sudo systemctl disable nginx 2>/dev/null || true
sudo systemctl restart runpilot-server
```

### B. Verify Standalone Router Listening State
```bash
sudo ss -tulpn | grep runpilot-server
# Expected: tcp LISTEN 0.0.0.0:80
```

---

## 2. Administrator Credential Recovery via CLI

If you forgot your admin password, need to change your admin email, or want to update credentials without web access:

```bash
# Reset password directly from VPS command line
/opt/runpilot/bin/runpilot-server -reset-user admin -new-pass "YourNewSecurePass2026!"

# Reset email address
/opt/runpilot/bin/runpilot-server -reset-user admin -new-email "admin@runpilot.org"

# Reset username
/opt/runpilot/bin/runpilot-server -reset-user admin -new-username "superadmin"
```

---

## 3. Preflight Diagnostics

If `installer/preflight.sh` fails:

### A. Insufficient Memory or Swap
- Check available RAM: `free -m`
- If you have 1GB RAM on a VPS, enable a swapfile:
  ```bash
  sudo fallocate -l 2G /swapfile
  sudo chmod 600 /swapfile
  sudo mkswap /swapfile
  sudo swapon /swapfile
  ```

### B. Missing systemd
- RunPilot relies on native Linux process control via systemd (`PID 1`).
- Verify systemd state: `ps -p 1 -o comm=` (should output `systemd`).
- If running inside a minimal container without an init system, run RunPilot on a standard Linux VM or VPS.

---

## 4. Application Process Diagnostics

### A. View Application Process Status
Check the generated systemd unit:
```bash
sudo systemctl status runpilot-app-<id>.service
```

### B. View Real-Time Application Logs
Tail process logs from systemd journal:
```bash
sudo journalctl -u runpilot-app-<id>.service -f -n 100
```
Or check the application's shared log directory:
```bash
tail -f /srv/runpilot/apps/<id>/shared/logs/app.log
```

### C. Port Conflict / Address in Use
If an application fails to start with `EADDRINUSE`:
```bash
sudo ss -tulpn | grep :<port>
```
Assign a free internal port in RunPilot application settings.

---

## 5. Deployment Engine Troubleshooting

### A. Node.js Build Failures
- Verify `package.json` syntax and dependencies.
- Check node release workspace:
  ```bash
  ls -la /srv/runpilot/apps/<id>/releases/<release_id>
  ```

### B. Python Virtual Environment Failures
- Ensure `requirements.txt` contains compatible package versions.
- Test activating the isolated virtual environment:
  ```bash
  source /srv/runpilot/apps/<id>/releases/<release_id>/.venv/bin/activate
  python -m pip list
  ```
