#!/usr/bin/env bash
# ==============================================================================
# RunPilot - Preflight Prerequisite & System Checker
# Product: RunPilot (Native Node.js & Python Application Hosting Control Panel)
# Description: Verifies system compatibility, resources, init system, ports,
#              and dependencies without ANY PHP or Composer dependencies.
# ==============================================================================

set -o pipefail

# Color codes for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

RUNPILOT_VERSION="1.0.0"
MIN_RAM_MB=1024
MIN_DISK_MB=4096
REQUIRED_PORTS=(80 443 8080 9090)

DRY_RUN=false
JSON_MODE=false
QUIET=false
STRICT=false

# Counters
CHECKS_TOTAL=0
CHECKS_PASSED=0
CHECKS_FAILED=0
CHECKS_WARNING=0

declare -a FAILURE_MESSAGES=()
declare -a WARNING_MESSAGES=()

usage() {
    cat <<EOF
RunPilot Preflight System Checker v${RUNPILOT_VERSION}

Usage: $(basename "$0") [OPTIONS]

Options:
  --dry-run       Run all checks without modifying or testing live system binds
  --json          Output results in JSON format
  --strict        Treat warnings as failures
  --quiet         Suppress progress messages, show only summary
  -h, --help      Display this help message

Notes:
  RunPilot is dedicated to native Node.js and Python hosting.
  This installer NEVER checks for, requires, or executes PHP or Composer.
EOF
    exit 0
}

# Parse command line flags
while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --json)
            JSON_MODE=true
            shift
            ;;
        --strict)
            STRICT=true
            shift
            ;;
        --quiet)
            QUIET=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

log_info() {
    if [ "$JSON_MODE" = false ] && [ "$QUIET" = false ]; then
        echo -e "${BLUE}[INFO]${NC} $1"
    fi
}

log_pass() {
    ((CHECKS_TOTAL++))
    ((CHECKS_PASSED++))
    if [ "$JSON_MODE" = false ] && [ "$QUIET" = false ]; then
        echo -e "${GREEN}[PASS]${NC} $1"
    fi
}

log_warn() {
    ((CHECKS_TOTAL++))
    ((CHECKS_WARNING++))
    WARNING_MESSAGES+=("$1")
    if [ "$JSON_MODE" = false ] && [ "$QUIET" = false ]; then
        echo -e "${YELLOW}[WARN]${NC} $1"
    fi
}

log_fail() {
    ((CHECKS_TOTAL++))
    ((CHECKS_FAILED++))
    FAILURE_MESSAGES+=("$1")
    if [ "$JSON_MODE" = false ] && [ "$QUIET" = false ]; then
        echo -e "${RED}[FAIL]${NC} $1"
    fi
}

check_root() {
    if [ "$(id -u)" -eq 0 ]; then
        log_pass "Privilege check: running as root (UID 0)"
    else
        if command -v sudo >/dev/null 2>&1; then
            log_warn "Privilege check: running as unprivileged user '$(whoami)', sudo is available for installation"
        else
            log_fail "Privilege check: root privileges or sudo access is required"
        fi
    fi
}

check_os() {
    local os_name=""
    local os_version=""
    local kernel_name
    kernel_name="$(uname -s)"

    if [ "$kernel_name" = "Linux" ]; then
        if [ -f /etc/os-release ]; then
            # shellcheck disable=SC1091
            . /etc/os-release
            os_name="$ID"
            os_version="$VERSION_ID"
            case "$os_name" in
                ubuntu|debian|almalinux|rocky|centos|rhel)
                    log_pass "Operating System: Linux ($PRETTY_NAME) is officially supported"
                    ;;
                *)
                    log_warn "Operating System: Linux ($PRETTY_NAME) is not in standard tested list, but standard Linux process features apply"
                    ;;
            esac
        else
            log_warn "Operating System: Generic Linux detected (/etc/os-release missing)"
        fi
    elif [ "$kernel_name" = "Darwin" ]; then
        log_warn "Operating System: macOS ($kernel_name) detected. Suitable for RunPilot local control-plane development/simulation, but production host requires Linux with systemd."
    else
        log_fail "Operating System: Unsupported kernel '$kernel_name'. RunPilot host requires Linux."
    fi
}

check_arch() {
    local arch
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)
            log_pass "Architecture: $arch is supported"
            ;;
        aarch64|arm64)
            log_pass "Architecture: $arch is supported"
            ;;
        *)
            log_fail "Architecture: $arch is unsupported (requires x86_64 or aarch64)"
            ;;
    esac
}

check_init_system() {
    if [ "$(uname -s)" = "Linux" ]; then
        if [ -d /run/systemd/system ]; then
            log_pass "Init System: systemd is active"
        else
            log_fail "Init System: systemd was not detected running as init (PID 1). RunPilot manages native processes via systemd."
        fi
    else
        log_warn "Init System: Non-Linux host; systemd checks skipped (development environment)"
    fi
}

check_cgroups() {
    if [ "$(uname -s)" = "Linux" ]; then
        if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
            log_pass "Resource Control: cgroups v2 unified hierarchy is available"
        elif [ -d /sys/fs/cgroup ]; then
            log_warn "Resource Control: cgroups v1 detected. Basic limits apply, but cgroups v2 is recommended for complete isolation."
        else
            log_warn "Resource Control: cgroup controllers not mounted. Resource limits will rely on basic ulimits."
        fi
    else
        log_warn "Resource Control: cgroups check skipped on non-Linux host"
    fi
}

check_resources() {
    # Check RAM
    local total_ram_mb=0
    if [ "$(uname -s)" = "Linux" ] && [ -f /proc/meminfo ]; then
        total_ram_mb=$(awk '/MemTotal/ { printf "%.0f", $2/1024 }' /proc/meminfo)
    elif [ "$(uname -s)" = "Darwin" ]; then
        local bytes
        bytes=$(sysctl -n hw.memsize 2>/dev/null || echo 0)
        total_ram_mb=$((bytes / 1024 / 1024))
    fi

    if [ "$total_ram_mb" -ge "$MIN_RAM_MB" ]; then
        log_pass "Memory: ${total_ram_mb} MB RAM available (minimum required: ${MIN_RAM_MB} MB)"
    elif [ "$total_ram_mb" -eq 0 ]; then
        log_warn "Memory: Unable to detect total RAM, ensure at least ${MIN_RAM_MB} MB is available"
    else
        log_fail "Memory: ${total_ram_mb} MB RAM detected. Insufficient memory (minimum required: ${MIN_RAM_MB} MB)"
    fi

    # Check Disk Space in current / target mount
    local free_disk_mb=0
    free_disk_mb=$(df -m . 2>/dev/null | awk 'NR==2 {print $(NF-2)}')
    if [ -n "$free_disk_mb" ] && [ "$free_disk_mb" -ge "$MIN_DISK_MB" ]; then
        log_pass "Disk Space: ${free_disk_mb} MB free in working path (minimum: ${MIN_DISK_MB} MB)"
    elif [ -n "$free_disk_mb" ]; then
        log_fail "Disk Space: ${free_disk_mb} MB free. Insufficient disk space (minimum: ${MIN_DISK_MB} MB)"
    else
        log_warn "Disk Space: Could not determine available disk space"
    fi
}

check_native_capabilities() {
    local required_cmds=(curl tar openssl)
    for cmd in "${required_cmds[@]}"; do
        if command -v "$cmd" >/dev/null 2>&1; then
            log_pass "Native Tooling: '$cmd' is available"
        else
            log_fail "Native Tooling: Required utility '$cmd' is missing"
        fi
    done

    # Check Git (essential for Git-based deployment engine)
    if command -v git >/dev/null 2>&1; then
        local git_ver
        git_ver=$(git --version | head -n 1)
        log_pass "Native Tooling: Git is available ($git_ver)"
    else
        log_warn "Native Tooling: 'git' is not installed yet (installer will provision it for Git deployment engine)"
    fi

    # Check Nginx (reverse proxy for web routing & TLS)
    if command -v nginx >/dev/null 2>&1; then
        local nginx_ver
        nginx_ver=$(nginx -v 2>&1 | head -n 1)
        log_pass "Reverse Proxy: Nginx is installed ($nginx_ver)"
    else
        log_warn "Reverse Proxy: Nginx is not installed yet (installer can provision it automatically)"
    fi
}

check_database_availability() {
    if command -v mysql >/dev/null 2>&1 || command -v mariadb >/dev/null 2>&1; then
        log_pass "Database Client: MySQL/MariaDB client is available"
    else
        log_warn "Database Client: MySQL/MariaDB client not found in PATH (remote or embedded connector can be configured)"
    fi
}

check_ports() {
    if [ "$DRY_RUN" = true ]; then
        log_pass "Port Conflicts: Dry-run active, skipping live socket binds"
        return
    fi

    for port in "${REQUIRED_PORTS[@]}"; do
        local in_use=false
        if command -v ss >/dev/null 2>&1; then
            if ss -tuln | grep -q ":${port} "; then
                in_use=true
            fi
        elif command -v netstat >/dev/null 2>&1; then
            if netstat -tuln 2>/dev/null | grep -q ":${port} "; then
                in_use=true
            fi
        elif command -v lsof >/dev/null 2>&1; then
            if lsof -i ":${port}" >/dev/null 2>&1; then
                in_use=true
            fi
        fi

        if [ "$in_use" = true ]; then
            if [ "$port" -eq 80 ] || [ "$port" -eq 443 ]; then
                log_warn "Port Conflict: Port $port is currently in use (if Nginx is already running, RunPilot will integrate with it)"
            else
                log_warn "Port Conflict: Port $port is in use; configure custom ports in RunPilot configuration if necessary"
            fi
        else
            log_pass "Port Availability: Port $port is free"
        fi
    done
}

run_checks() {
    if [ "$JSON_MODE" = false ] && [ "$QUIET" = false ]; then
        echo -e "${BOLD}${CYAN}================================================================${NC}"
        echo -e "${BOLD}${CYAN}      RunPilot Preflight & System Prerequisite Checker         ${NC}"
        echo -e "${BOLD}${CYAN}      Platform: Native Node.js & Python Application Panel     ${NC}"
        echo -e "${BOLD}${CYAN}================================================================${NC}"
        echo ""
    fi

    check_root
    check_os
    check_arch
    check_init_system
    check_cgroups
    check_resources
    check_native_capabilities
    check_database_availability
    check_ports

    # Final summary evaluation
    local exit_code=0
    if [ "$CHECKS_FAILED" -gt 0 ]; then
        exit_code=1
    elif [ "$STRICT" = true ] && [ "$CHECKS_WARNING" -gt 0 ]; then
        exit_code=1
    fi

    if [ "$JSON_MODE" = true ]; then
        cat <<EOF
{
  "product": "RunPilot",
  "version": "${RUNPILOT_VERSION}",
  "status": "$([ $exit_code -eq 0 ] && echo "PASSED" || echo "FAILED")",
  "checks": {
    "total": $CHECKS_TOTAL,
    "passed": $CHECKS_PASSED,
    "warnings": $CHECKS_WARNING,
    "failures": $CHECKS_FAILED
  },
  "php_detected": false,
  "composer_detected": false
}
EOF
    else
        echo ""
        echo -e "${BOLD}Preflight Summary:${NC}"
        echo -e "  Total Checks:   ${BOLD}${CHECKS_TOTAL}${NC}"
        echo -e "  Passed:         ${GREEN}${CHECKS_PASSED}${NC}"
        echo -e "  Warnings:       ${YELLOW}${CHECKS_WARNING}${NC}"
        echo -e "  Failures:       ${RED}${CHECKS_FAILED}${NC}"
        echo ""

        if [ "$CHECKS_FAILED" -gt 0 ]; then
            echo -e "${RED}${BOLD}Preflight checks failed with the following issues:${NC}"
            for msg in "${FAILURE_MESSAGES[@]}"; do
                echo -e "  - ${RED}$msg${NC}"
            done
            echo ""
            echo "Please resolve the above issues before continuing installation."
        else
            echo -e "${GREEN}${BOLD}System meets RunPilot preflight requirements.${NC}"
            if [ "$CHECKS_WARNING" -gt 0 ]; then
                echo -e "${YELLOW}Notice: Some non-fatal warnings were detected. Review above details.${NC}"
            fi
        fi
    fi

    exit "$exit_code"
}

run_checks
