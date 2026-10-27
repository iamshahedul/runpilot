#!/usr/bin/env bash
# ==============================================================================
# Implementation Gate Test: Verify ABSOLUTE NO PHP / NO COMPOSER Requirement
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BOLD}${CYAN}================================================================${NC}"
echo -e "${BOLD}${CYAN}   RUNPILOT GATE 1 TEST: NO PHP OR COMPOSER DETECTION AUDIT    ${NC}"
echo -e "${BOLD}${CYAN}================================================================${NC}"

TESTS_PASSED=0
TESTS_FAILED=0

assert_true() {
    local desc="$1"
    local condition="$2"
    if eval "$condition"; then
        echo -e "${GREEN}[PASS]${NC} $desc"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}[FAIL]${NC} $desc"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# -----------------------------------------------------------------------------
# Test 1: Static Code Audit - Forbidden commands and terms in installer files
# -----------------------------------------------------------------------------
echo ""
echo -e "${BOLD}--- Test 1: Static Code Audit in Installer Scripts ---${NC}"

FORBIDDEN_PATTERNS=(
    'which php\b'
    'command -v php\b'
    'php -v'
    'php --version'
    'which composer\b'
    'command -v composer\b'
    'composer --version'
    'composer install'
    'composer update'
    'php-fpm'
    'php-cli'
    'composer.json'
    'composer.lock'
    'artisan'
)

STATIC_FAIL=0
for script in "${ROOT_DIR}/installer/preflight.sh" "${ROOT_DIR}/installer/install.sh"; do
    if [ ! -f "$script" ]; then
        echo -e "${RED}[FAIL] Script not found: $script${NC}"
        STATIC_FAIL=$((STATIC_FAIL + 1))
        continue
    fi
    for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
        if grep -E -n "$pattern" "$script" >/dev/null 2>&1; then
            echo -e "${RED}[VIOLATION] Forbidden pattern '$pattern' found in $script:${NC}"
            grep -E -n "$pattern" "$script"
            STATIC_FAIL=$((STATIC_FAIL + 1))
        fi
    done
done

assert_true "Static scan: zero forbidden PHP/Composer invocation patterns in installer scripts" "[ $STATIC_FAIL -eq 0 ]"

# -----------------------------------------------------------------------------
# Test 2: Execution Trap Test
# If preflight.sh or install.sh tries to call php or composer, mock binaries will record violation
# -----------------------------------------------------------------------------
echo ""
echo -e "${BOLD}--- Test 2: Dynamic Execution Trap Test ---${NC}"

MOCK_TRAP_DIR=$(mktemp -d -t runpilot-trap-XXXXXX)
TRAP_LOG="${MOCK_TRAP_DIR}/invocations.log"
touch "$TRAP_LOG"

create_trap() {
    local binary_name="$1"
    cat <<EOF > "${MOCK_TRAP_DIR}/${binary_name}"
#!/usr/bin/env bash
echo "ILLEGAL INVOCATION of ${binary_name} with args: \$*" >> "${TRAP_LOG}"
exit 99
EOF
    chmod +x "${MOCK_TRAP_DIR}/${binary_name}"
}

create_trap "php"
create_trap "php8"
create_trap "php8.1"
create_trap "php8.2"
create_trap "php8.3"
create_trap "composer"
create_trap "composer.phar"

# Run preflight with trap directory prepended to PATH
TRAP_EXIT=0
PATH="${MOCK_TRAP_DIR}:${PATH}" bash "${ROOT_DIR}/installer/preflight.sh" --dry-run --quiet >/dev/null 2>&1 || TRAP_EXIT=$?

assert_true "Trap Test: preflight.sh completed without touching trapped php/composer binaries" "[ ! -s '$TRAP_LOG' ]"

# Run install.sh with trap directory prepended to PATH
PATH="${MOCK_TRAP_DIR}:${PATH}" bash "${ROOT_DIR}/installer/install.sh" --dry-run --skip-preflight >/dev/null 2>&1 || true

assert_true "Trap Test: install.sh completed without touching trapped php/composer binaries" "[ ! -s '$TRAP_LOG' ]"

rm -rf "$MOCK_TRAP_DIR"

# -----------------------------------------------------------------------------
# Test 3: Clean Environment Test (Zero PHP or Composer in PATH)
# -----------------------------------------------------------------------------
echo ""
echo -e "${BOLD}--- Test 3: Execution in Clean Environment without PHP or Composer ---${NC}"

CLEAN_BIN_DIR=$(mktemp -d -t runpilot-clean-XXXXXX)
# Link essential system tools only (strictly excluding php and composer)
for tool in bash sh uname df sysctl awk grep sed cat mkdir chmod rm touch curl tar openssl id whoami cut tr sudo git head; do
    tool_path=$(command -v "$tool" 2>/dev/null || true)
    if [ -n "$tool_path" ] && [ -x "$tool_path" ]; then
        ln -s "$tool_path" "${CLEAN_BIN_DIR}/$tool" 2>/dev/null || true
    fi
done

CLEAN_OUTPUT=$(PATH="${CLEAN_BIN_DIR}" bash "${ROOT_DIR}/installer/preflight.sh" --dry-run --json)
rm -rf "$CLEAN_BIN_DIR"

# Verify JSON output validity and that status is PASSED or warnings only (no PHP failures)
assert_true "Clean Env: Preflight returns valid JSON" "echo '$CLEAN_OUTPUT' | grep -q '\"product\": \"RunPilot\"'"
assert_true "Clean Env: Preflight reports php_detected: false" "echo '$CLEAN_OUTPUT' | grep -q '\"php_detected\": false'"
assert_true "Clean Env: Preflight reports composer_detected: false" "echo '$CLEAN_OUTPUT' | grep -q '\"composer_detected\": false'"
assert_true "Clean Env: Preflight reports 0 failures in dry-run" "echo '$CLEAN_OUTPUT' | grep -q '\"failures\": 0'"

# -----------------------------------------------------------------------------
# Final Gate Evaluation
# -----------------------------------------------------------------------------
echo ""
echo -e "${BOLD}================================================================${NC}"
echo -e "Total Gate Checks Passed: ${GREEN}${TESTS_PASSED}${NC}"
echo -e "Total Gate Checks Failed: ${RED}${TESTS_FAILED}${NC}"
echo -e "${BOLD}================================================================${NC}"

if [ "$TESTS_FAILED" -eq 0 ]; then
    echo -e "${GREEN}${BOLD}GATE 1 PASSED: Strict compliance with NO PHP OR COMPOSER requirement verified!${NC}"
    exit 0
else
    echo -e "${RED}${BOLD}GATE 1 FAILED: Violations found! Resolve before proceeding.${NC}"
    exit 1
fi
