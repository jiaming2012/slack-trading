#!/bin/sh
# Headless demo-covered-call regression orchestrator (repair-python-test-suite).
#
# Boots the Go trading server in development mode as a background process,
# waits for the Twirp RPC port (5051) to accept connections (ignoring the
# known, non-fatal REST port-8080 bind conflict per the CLAUDE.md gotcha),
# then runs tests/test_demo_covered_call.py with its harness flag set
# (DEMO_COVERED_CALL_HARNESS=1). The test module itself runs
# demos.demo_covered_call end-to-end (AAPL 2025-01-01..2025-03-01, ~10-15 min
# of Polygon-backed simulation) and pins its reference metrics.
#
# Port discipline: NEVER kills a server it did not start. If something else
# holds Twirp 5051, this script WAITS (up to DEMO_CC_PORT_WAIT_S, default
# 20 min) for the port to free, then refuses if it never does.
# Teardown kills only the process group this script created, on every exit
# path. The script's exit code equals the pytest exit code.
#
# POSIX sh (dash) compatible: `set -eu`, no bash-only pipefail, no arrays.
#
# Usage:
#   ./run_demo_covered_call_test.sh
#
# Environment overrides (all optional):
#   TRADING_PROJECT_DIR     repo root that holds .env      (default: resolved repo root)
#   DEMO_CC_TWIRP_HOST      Twirp server URL               (default: http://127.0.0.1:5051)
#   DEMO_CC_BOOT_TIMEOUT_S  server-boot readiness wait     (default: 90)
#   DEMO_CC_PORT_WAIT_S     wait for a busy 5051 to free   (default: 1200)
#   DEMO_CC_KILL_GRACE_S    SIGTERM->SIGKILL grace secs    (default: 10)
#   PYTHON                  grodt python interpreter       (default: ~/miniconda3/envs/grodt/bin/python)

set -eu

# --- Resolve paths -----------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"          # src/clients/python
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"      # repo (or worktree) root
CMD_DIR="$REPO_ROOT/cmd"

PYTHON="${PYTHON:-$HOME/miniconda3/envs/grodt/bin/python}"
TWIRP_HOST="${DEMO_CC_TWIRP_HOST:-http://127.0.0.1:5051}"
BOOT_TIMEOUT_S="${DEMO_CC_BOOT_TIMEOUT_S:-90}"
PORT_WAIT_S="${DEMO_CC_PORT_WAIT_S:-1200}"
KILL_GRACE_S="${DEMO_CC_KILL_GRACE_S:-10}"

# Parse host/port out of the Twirp URL for the readiness poll.
HOST_PORT="${TWIRP_HOST#http://}"
HOST_PORT="${HOST_PORT#https://}"
HOST_ONLY="${HOST_PORT%%:*}"
PORT_ONLY="${HOST_PORT##*:}"

# Server env: mirror cmd/run-dev.sh's resolution so the worktree's own code and
# config are what gets tested. .env must live at $TRADING_PROJECT_DIR/.env.
TRADING_PROJECT_DIR="${TRADING_PROJECT_DIR:-$REPO_ROOT}"
OPTIONS_CONFIG_PATH="${OPTIONS_CONFIG_PATH:-$REPO_ROOT/src/go/${OPTIONS_CONFIG_FILE:-options-config.yaml}}"

SERVER_LOG="$(mktemp "${TMPDIR:-/tmp}/demo-cc-server.XXXXXX.log")"
SERVER_PGID=""

# --- Teardown: runs on EVERY exit path (pass, fail, or interrupt) ------------
# Kills only the process group we created (surgical: never touches a server
# started by another process). SIGTERM first, then SIGKILL after a grace period.
teardown() {
    if [ -n "$SERVER_PGID" ]; then
        # Negative PID => signal the whole process group. Fall back to the
        # positive PID for the edge case where boot failed before the setsid
        # child established its own group.
        kill -TERM "-$SERVER_PGID" 2>/dev/null || kill -TERM "$SERVER_PGID" 2>/dev/null || true
        i=0
        while [ "$i" -lt "$KILL_GRACE_S" ]; do
            if ! kill -0 "-$SERVER_PGID" 2>/dev/null; then
                break
            fi
            sleep 1
            i=$((i + 1))
        done
        if kill -0 "-$SERVER_PGID" 2>/dev/null; then
            echo "[demo-cc] server group $SERVER_PGID did not exit after ${KILL_GRACE_S}s; sending SIGKILL" >&2
            kill -KILL "-$SERVER_PGID" 2>/dev/null || true
        fi
        echo "[demo-cc] server group $SERVER_PGID torn down" >&2
    fi
    rm -f "$SERVER_LOG" 2>/dev/null || true
}
trap teardown EXIT INT TERM

# --- Port discipline: wait (never kill) if something else holds Twirp 5051 ---
# We must never adopt (or later kill) a server we did not start. If the port is
# busy, poll until it frees (bounded), then refuse if it never does.
if nc -z "$HOST_ONLY" "$PORT_ONLY" 2>/dev/null; then
    echo "[demo-cc] ${HOST_ONLY}:${PORT_ONLY} is busy (foreign server?). Waiting up to ${PORT_WAIT_S}s for it to free — will NOT kill it." >&2
    waited=0
    while nc -z "$HOST_ONLY" "$PORT_ONLY" 2>/dev/null; do
        if [ "$waited" -ge "$PORT_WAIT_S" ]; then
            echo "ERROR: ${HOST_ONLY}:${PORT_ONLY} still busy after ${PORT_WAIT_S}s." >&2
            echo "       Refusing to boot a second server or adopt/kill one this script did not start." >&2
            echo "       Stop the other server (or point DEMO_CC_TWIRP_HOST elsewhere) and retry." >&2
            exit 1
        fi
        sleep 5
        waited=$((waited + 5))
    done
    echo "[demo-cc] port freed after ${waited}s; proceeding." >&2
fi

if [ ! -f "$TRADING_PROJECT_DIR/.env" ]; then
    echo "ERROR: no .env found at ${TRADING_PROJECT_DIR}/.env (required to boot the server)." >&2
    echo "       Set TRADING_PROJECT_DIR to a repo root that has a .env, or copy one in." >&2
    exit 1
fi

# --- Boot the Go server (dev mode) in its own session/process group ----------
echo "[demo-cc] booting Go server from ${CMD_DIR} (GO_ENV=development), log: ${SERVER_LOG}" >&2
GO_ENV=development \
TRADING_PROJECT_DIR="$TRADING_PROJECT_DIR" \
OPTIONS_CONFIG_PATH="$OPTIONS_CONFIG_PATH" \
OTEL_SDK_DISABLED=true \
    setsid sh -c "cd '$CMD_DIR' && exec go run ./main.go" >"$SERVER_LOG" 2>&1 &
SERVER_PGID="$!"   # setsid makes this pid the leader of a new process group
echo "[demo-cc] server process group: ${SERVER_PGID}" >&2

# --- Bounded readiness poll on Twirp 5051 ONLY (REST 8080 conflict ignored) --
ready=0
i=0
while [ "$i" -lt "$BOOT_TIMEOUT_S" ]; do
    # Aliveness check uses the positive PID: the new process GROUP only exists
    # once the setsid child has actually called setsid(), so checking
    # "-$SERVER_PGID" here races group creation on fast machines.
    if ! kill -0 "$SERVER_PGID" 2>/dev/null; then
        echo "ERROR: server process exited during boot. Last 30 log lines:" >&2
        tail -n 30 "$SERVER_LOG" >&2 || true
        exit 1
    fi
    if nc -z "$HOST_ONLY" "$PORT_ONLY" 2>/dev/null; then
        ready=1
        break
    fi
    sleep 1
    i=$((i + 1))
done

if [ "$ready" -ne 1 ]; then
    echo "ERROR: Twirp port ${PORT_ONLY} did not accept connections within ${BOOT_TIMEOUT_S}s." >&2
    echo "       Last 30 server log lines:" >&2
    tail -n 30 "$SERVER_LOG" >&2 || true
    exit 1   # trap teardown terminates the partially-started server
fi
echo "[demo-cc] Twirp ready on ${HOST_ONLY}:${PORT_ONLY} after ${i}s" >&2

# --- Run the demo regression module; capture its exit code -------------------
# The module's setUpClass runs demos.demo_covered_call as a subprocess against
# the server we just booted (~10-15 min: Polygon fetch + full simulation).
# Do not let a nonzero pytest exit trip `set -e` before we record it.
echo "[demo-cc] running tests/test_demo_covered_call.py (expect ~10-15 min) ..." >&2
set +e
OTEL_SDK_DISABLED=true \
DEMO_COVERED_CALL_HARNESS=1 \
    "$PYTHON" -m pytest "$SCRIPT_DIR/tests/test_demo_covered_call.py" -v
TEST_RC=$?
set -e

if [ "$TEST_RC" -eq 0 ]; then
    echo "[demo-cc] PASSED — demo ran end-to-end and all pinned reference metrics matched." >&2
else
    echo "[demo-cc] FAILED — pytest exit code ${TEST_RC}." >&2
fi

# teardown runs via trap; propagate the pytest exit code.
exit "$TEST_RC"
