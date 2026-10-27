#!/usr/bin/env bash
# ==============================================================================
# RunPilot - Master Server Installer
# Product: RunPilot (Native Node.js & Python Application Hosting Control Panel)
#
# Requirements Enforced:
# 1. Native Linux processes (systemd, unprivileged users, cgroups v2)
# 2. Dedicated to Node.js and Python hosting
# 3. ABSOLUTELY NO PHP OR COMPOSER DEPENDENCY, CHECK, OR INSTALLATION
# ==============================================================================

set -euo pipefail

RUNPILOT_VERSION="1.0.0"
INSTALL_PREFIX="/opt/runpilot"
DATA_PREFIX="/srv/runpilot"
CONFIG_DIR="/etc/runpilot"
LOG_DIR="/var/log/runpilot"

ADMIN_EMAIL="admin@runpilot.local"
ADMIN_PASS="RunPilot2026!"
DB_HOST="127.0.0.1"
DB_PORT="3306"
DB_NAME="runpilot"
DB_USER="runpilot"
DB_PASS=""
SERVER_PORT="80"
AGENT_PORT="9090"
SKIP_PREFLIGHT=false
DRY_RUN=false

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PREFLIGHT_SCRIPT="${SCRIPT_DIR}/preflight.sh"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log() {
    echo -e "${CYAN}[RunPilot]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_err() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

banner() {
    cat <<'EOF'
  ____              ____  _ _       _   
 |  _ \ _   _ _ __ |  _ \(_) | ___ | |_ 
 | |_) | | | | '_ \| |_) | | |/ _ \| __|
 |  _ <| |_| | | | |  __/| | | (_) | |_ 
 |_| \_\\__,_|_| |_|_|   |_|_|\___/ \__|

 Modern Node.js & Python Hosting Control Panel
 Native Linux Processes • systemd • cgroups v2
EOF
    echo -e " Version: ${BOLD}${RUNPILOT_VERSION}${NC}"
    echo "=========================================================="
}

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Options:
  --dry-run              Simulate installation steps without applying changes
  --skip-preflight       Skip system prerequisite checks (not recommended)
  --install-dir DIR      Installation path (default: /opt/runpilot)
  --data-dir DIR         Application data path (default: /srv/runpilot)
  --admin-email EMAIL    Administrator email (default: admin@runpilot.local)
  --admin-pass PASS      Administrator password (default: RunPilot2026!)
  --port PORT            Control plane HTTP port (default: 80)
  --agent-port PORT      Host agent internal API port (default: 9090)
  --db-host HOST         Database host (default: 127.0.0.1)
  --db-port PORT         Database port (default: 3306)
  --db-name NAME         Database name (default: runpilot)
  --db-user USER         Database username (default: runpilot)
  -h, --help             Show this help message

Notes:
  RunPilot is a 100% standalone, dedicated Node.js & Python control panel.
  It does NOT require, check, or install PHP, Composer, Nginx, or external DNS.
EOF
    exit 0
}

# Parse options
while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --skip-preflight)
            SKIP_PREFLIGHT=true
            shift
            ;;
        --install-dir)
            INSTALL_PREFIX="$2"
            shift 2
            ;;
        --data-dir)
            DATA_PREFIX="$2"
            shift 2
            ;;
        --admin-email)
            ADMIN_EMAIL="$2"
            shift 2
            ;;
        --admin-pass)
            ADMIN_PASS="$2"
            shift 2
            ;;
        --port)
            SERVER_PORT="$2"
            shift 2
            ;;
        --agent-port)
            AGENT_PORT="$2"
            shift 2
            ;;
        --db-host)
            DB_HOST="$2"
            shift 2
            ;;
        --db-port)
            DB_PORT="$2"
            shift 2
            ;;
        --db-name)
            DB_NAME="$2"
            shift 2
            ;;
        --db-user)
            DB_USER="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            log_err "Unknown option: $1"
            exit 1
            ;;
    esac
done

run_preflight() {
    if [ "$SKIP_PREFLIGHT" = true ]; then
        log_warn "Skipping preflight checks as requested by operator."
        return 0
    fi

    log "Executing system preflight checks..."
    if [ -f "$PREFLIGHT_SCRIPT" ]; then
        if bash "$PREFLIGHT_SCRIPT"; then
            log_success "System preflight checks passed."
        else
            log_err "Preflight checks failed. Please fix system prerequisites or run with --dry-run."
            if [ "$DRY_RUN" = false ]; then
                exit 1
            fi
        fi
    else
        log_warn "preflight.sh not found at ${PREFLIGHT_SCRIPT}. Continuing with basic validation."
    fi
}

create_system_directories() {
    log "Creating system directories..."
    local dirs=(
        "${INSTALL_PREFIX}/bin"
        "${INSTALL_PREFIX}/web"
        "${CONFIG_DIR}"
        "${LOG_DIR}"
        "${DATA_PREFIX}/apps"
        "${DATA_PREFIX}/runtimes"
        "${DATA_PREFIX}/shared"
        "${DATA_PREFIX}/backups"
    )

    for dir in "${dirs[@]}"; do
        if [ "$DRY_RUN" = true ]; then
            log "[DRY RUN] Would create directory: $dir (mode 0750)"
        else
            mkdir -p "$dir"
            chmod 0755 "$dir" 2>/dev/null || true
        fi
    done
}

create_service_account() {
    log "Checking RunPilot service account..."
    if id -u runpilot >/dev/null 2>&1; then
        log "System user 'runpilot' already exists."
    else
        if [ "$DRY_RUN" = true ]; then
            log "[DRY RUN] Would create system user 'runpilot'"
        else
            if command -v useradd >/dev/null 2>&1; then
                useradd --system --shell /bin/false --home-dir "${DATA_PREFIX}" runpilot 2>/dev/null || true
                log_success "Created system user 'runpilot'."
            fi
        fi
    fi
}

generate_agent_auth_secret() {
    log "Generating secure internal agent token..."
    if [ "$DRY_RUN" = true ]; then
        log "[DRY RUN] Would generate 256-bit cryptographic secret for agent authentication"
        return
    fi

    local secret_file="${CONFIG_DIR}/agent.secret"
    if [ ! -f "$secret_file" ]; then
        if command -v openssl >/dev/null 2>&1; then
            openssl rand -hex 32 > "$secret_file"
        else
            head -c 32 /dev/urandom | od -A n -v -t x1 | tr -d ' \n' > "$secret_file"
        fi
        chmod 0644 "$secret_file" 2>/dev/null || true
        log_success "Generated agent secret key at ${secret_file}"
    else
        log "Agent secret already present at ${secret_file}."
    fi
}

generate_config_files() {
    log "Generating configuration templates..."
    local server_conf="${CONFIG_DIR}/server.json"
    local agent_conf="${CONFIG_DIR}/agent.json"

    if [ "$DRY_RUN" = true ]; then
        log "[DRY RUN] Would generate ${server_conf} and ${agent_conf}"
        return
    fi

    if [ ! -f "$server_conf" ]; then
        cat <<EOF > "$server_conf"
{
  "server": {
    "host": "0.0.0.0",
    "port": ${SERVER_PORT},
    "web_root": "${INSTALL_PREFIX}/web",
    "jwt_secret": "$(openssl rand -hex 32 2>/dev/null || echo 'runpilot-default-jwt-secret-key-replace')",
    "data_dir": "${DATA_PREFIX}"
  },
  "database": {
    "driver": "sqlite",
    "host": "${DB_HOST}",
    "port": ${DB_PORT},
    "name": "${DB_NAME}",
    "user": "${DB_USER}",
    "password": "${DB_PASS}",
    "sqlite_path": "${DATA_PREFIX}/runpilot.db"
  },
  "agent": {
    "endpoint": "http://127.0.0.1:${AGENT_PORT}",
    "secret_file": "${CONFIG_DIR}/agent.secret"
  }
}
EOF
        chmod 0644 "$server_conf"
        log_success "Generated control plane configuration at ${server_conf}"
    fi

    if [ ! -f "$agent_conf" ]; then
        cat <<EOF > "$agent_conf"
{
  "agent": {
    "host": "127.0.0.1",
    "port": ${AGENT_PORT},
    "secret_file": "${CONFIG_DIR}/agent.secret",
    "data_dir": "${DATA_PREFIX}",
    "apps_dir": "${DATA_PREFIX}/apps",
    "runtimes_dir": "${DATA_PREFIX}/runtimes",
    "log_dir": "${LOG_DIR}",
    "nginx_sites_dir": "/etc/nginx/conf.d",
    "systemd_unit_prefix": "runpilot-app"
  }
}
EOF
        chmod 0644 "$agent_conf"
        log_success "Generated host agent configuration at ${agent_conf}"
    fi

    # Write administrator bootstrap environment credentials
    local server_env="${CONFIG_DIR}/server.env"
    cat <<EOF > "$server_env"
RUNPILOT_ADMIN_EMAIL="${ADMIN_EMAIL}"
RUNPILOT_ADMIN_PASSWORD="${ADMIN_PASS}"
EOF
    chmod 0600 "$server_env"
    log_success "Saved administrator credentials to ${server_env}"

    # Ensure runpilot user owns all required directories and files
    chown -R runpilot:runpilot "${CONFIG_DIR}" "${INSTALL_PREFIX}" "${DATA_PREFIX}" "${LOG_DIR}" 2>/dev/null || true
}

install_binaries_and_assets() {
    log "Installing dashboard assets, migrations, and systemd units..."
    if [ -d "${SCRIPT_DIR}/../control-plane/web" ]; then
        mkdir -p "${INSTALL_PREFIX}/web"
        cp -r "${SCRIPT_DIR}/../control-plane/web"/* "${INSTALL_PREFIX}/web/" 2>/dev/null || true
    fi
    if [ -d "${SCRIPT_DIR}/../control-plane/migrations" ]; then
        mkdir -p "${INSTALL_PREFIX}/migrations"
        cp -r "${SCRIPT_DIR}/../control-plane/migrations"/* "${INSTALL_PREFIX}/migrations/" 2>/dev/null || true
    fi

    # Install systemd unit files
    if [ -f "${SCRIPT_DIR}/templates/runpilot-server.service" ]; then
        cp "${SCRIPT_DIR}/templates/runpilot-server.service" /etc/systemd/system/
    fi
    if [ -f "${SCRIPT_DIR}/templates/runpilot-agent.service" ]; then
        cp "${SCRIPT_DIR}/templates/runpilot-agent.service" /etc/systemd/system/
    fi
    systemctl daemon-reload 2>/dev/null || true

    # Ensure binary has permissions to bind low ports (80/443)
    if [ -f "${INSTALL_PREFIX}/bin/runpilot-server" ] && command -v setcap >/dev/null 2>&1; then
        setcap 'cap_net_bind_service=+ep' "${INSTALL_PREFIX}/bin/runpilot-server" 2>/dev/null || true
    fi
    chown -R runpilot:runpilot "${INSTALL_PREFIX}" "${CONFIG_DIR}" "${DATA_PREFIX}" 2>/dev/null || true
}

main() {
    banner
    run_preflight
    create_system_directories
    create_service_account
    generate_agent_auth_secret
    generate_config_files
    install_binaries_and_assets

    echo ""
    echo -e "${GREEN}${BOLD}RunPilot installation setup completed successfully!${NC}"
    echo -e "Control Plane URL:      ${BOLD}http://<YOUR_IP>:${SERVER_PORT}${NC}"
    echo -e "Administrator Email:    ${BOLD}${ADMIN_EMAIL}${NC}"
    echo -e "Administrator Password: ${BOLD}${ADMIN_PASS}${NC}"
    echo -e "Host Agent Port:        ${BOLD}${AGENT_PORT}${NC}"
    echo -e "Configuration:          ${BOLD}${CONFIG_DIR}${NC}"
    echo -e "Application Data:       ${BOLD}${DATA_PREFIX}/apps${NC}"
    echo ""
    echo "To start services with systemd, run:"
    echo "  systemctl enable --now runpilot-agent"
    echo "  systemctl enable --now runpilot-server"
    echo ""
}

main "$@"
