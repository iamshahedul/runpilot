# RunPilot - Troubleshooting & Operational Diagnostics

This guide provides step-by-step diagnostic procedures for common operational situations.

---

## 1. Preflight Diagnostics

If `installer/preflight.sh` fails:

### A. Insufficient Memory or Swap
- Check available RAM: `free -m`
- If you have less than 1GB RAM on a VPS, enable a swapfile:
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

## 2. Application Process Diagnostics

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
Assign a free port in RunPilot application settings or let RunPilot auto-assign a dedicated internal upstream port.

---

## 3. Deployment Engine Troubleshooting

### A. Node.js Build Failures
- Verify `package.json` syntax and dependencies.
- Ensure the build command matches the framework (e.g. `npm run build` or `npm install --omit=dev`).
- Check node release workspace:
  ```bash
  ls -la /srv/runpilot/apps/<id>/releases/<release_id>
  ```

### B. Python Virtual Environment Failures
- Ensure `requirements.txt` or `pyproject.toml` contains compatible package versions.
- Test activating the isolated virtual environment:
  ```bash
  source /srv/runpilot/apps/<id>/releases/<release_id>/.venv/bin/activate
  python -m pip list
  ```

### C. Git Authentication Failures
- Ensure the deploy key or token is configured for private repositories.
- Test connection manually: `ssh -T git@github.com` using the app's dedicated deploy key.

---

## 4. Reverse Proxy & Nginx Diagnostics

### A. Verify Nginx Configuration
```bash
sudo nginx -t
```

### B. View Nginx Site Configuration
```bash
cat /etc/nginx/conf.d/runpilot-app-<id>.conf
```

### C. Reload Nginx Safely
```bash
sudo systemctl reload nginx
```
