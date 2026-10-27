# RunPilot - Security Architecture & Threat Model

Security is an essential design foundation of RunPilot. Below is the specification of security boundaries, isolation mechanics, and defense-in-depth measures.

---

## 1. Privilege Separation & User Isolation

1. **Applications Never Run as Root**:
   - Every deployed application is assigned its own dedicated system user and group (e.g. `runpilot-app-<id>`).
   - The user has no sudo privileges and cannot write to system directories (`/etc`, `/usr`, `/opt`, `/var`).
   - Applications have filesystem access strictly limited to their own `/srv/runpilot/apps/<id>` tree.

2. **Systemd Sandboxing Directives**:
   Every generated service unit enforces Linux kernel sandboxing:
   - `ProtectSystem=full`: Mounts `/usr`, `/boot`, and `/etc` read-only for the process.
   - `ProtectHome=true`: Makes `/home`, `/root`, and `/run/user` inaccessible.
   - `NoNewPrivileges=true`: Disallows child processes from gaining elevated privileges (e.g., via setuid binaries).
   - `PrivateTmp=true`: Allocates an isolated, private `/tmp` filesystem for the service namespace.

3. **cgroups v2 Resource Governance**:
   - `CPUQuota`: Limits CPU cores to prevent a compromised or infinite-looping application from starving other processes.
   - `MemoryMax`: Hard memory limit. Triggers OOM kill on the specific unit rather than destabilizing the host.
   - `TasksMax`: Limits maximum threads and spawned processes, preventing fork bombs.

---

## 2. Control Plane & Agent Communication Security

1. **Zero Arbitrary Command Execution**:
   - The Host Agent **never** exposes an endpoint to run arbitrary shell strings or bash scripts sent from the dashboard.
   - Every API endpoint executes a strictly typed Go handler with validated arguments (e.g., specific Git URL, commit hash, verified runtime version).

2. **Mutual Authentication & Replay Defense**:
   - Agent requests are signed and authenticated via an HMAC-SHA256 signature and shared secret token (`agent.secret`).
   - Requests include a timestamp and nonce to defend against replay attacks.

---

## 3. Storage of Secrets & Environment Variables

1. **Encryption at Rest**:
   - Application secrets and API credentials stored in MySQL/MariaDB are encrypted using AES-256-GCM.
   - The master encryption key is read from an environment variable or protected configuration file (`server.json`) and is never stored in the database alongside ciphertext.

2. **Safe Logging (Zero Leaks)**:
   - Environment variables defined in the control plane are masked in deployment logs.
   - Deploy keys and repository tokens are scrubbed from Git checkout logs before storage.

---

## 4. Web Application Security

1. **Authentication**:
   - Passwords hashed using bcrypt (cost factor 12) or Argon2id.
   - Rate limiting on `/api/v1/auth/login` to prevent credential stuffing.
   - JWT tokens signed with secure keys; refresh tokens tracked in database for instantaneous revocation.

2. **Input Validation & Path Traversal Prevention**:
   - All filesystem paths are canonicalized with `filepath.Clean` and validated against allowed prefixes.
   - Any path attempting to escape outside `/srv/runpilot/apps/<id>` is rejected with an audit alert.

3. **CSRF & Security Headers**:
   - All API endpoints validate `Origin` and `SameSite` cookie policies.
   - Standard HTTP security headers: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Content-Security-Policy`.
