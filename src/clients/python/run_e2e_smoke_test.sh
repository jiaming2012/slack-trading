#!/bin/sh
# Headless end-to-end Simulation smoke-test orchestrator (e2e-sim-smoke-test).
#
# Boots the Go trading server in development mode as a background process,
# waits for the Twirp RPC port (5051) to accept connections (ignoring the
# known, non-fatal REST port-8080 bind conflict per the CLAUDE.md gotcha),
# runs the pytest smoke module against it, and tears the server down on every
# exit path. The script's own exit code equals the pytest exit code, so it can
# serve as gate G5 for other OpenSpec changes and CI without a human watching.
#
# POSIX sh (dash) compatible: `set -eu`, no bash-only pipefail, no arrays.
#
# Usage:
#   ./run_e2e_smoke_test.sh
#
# Environment overrides (all optional):
#   TRADING_PROJECT_DIR   repo root that holds .env      (default: resolved repo root)
#   E2E_SMOKE_TWIRP_HOST  Twirp server URL               (default: http://127.0.0.1:5051)
#   E2E_SMOKE_BOOT_TIMEOUT_S  server-boot readiness wait  (default: 90)
#   E2E_SMOKE_KILL_GRACE_S    SIGTERM->SIGKILL grace secs (default: 10)
#   PYTHON                grodt python interpreter        (default: ~/miniconda3/envs/grodt/bin/python)

set -eu

# --- Resolve paths -----------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"          # src/clients/python
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"      # repo (or worktree) root
CMD_DIR="$REPO_ROOT/cmd"

PYTHON="${PYTHON:-$HOME/miniconda3/envs/grodt/bin/python}"
TWIRP_HOST="${E2E_SMOKE_TWIRP_HOST:-http://127.0.0.1:5051}"
BOOT_TIMEOUT_S="${E2E_SMOKE_BOOT_TIMEOUT_S:-90}"
KILL_GRACE_S="${E2E_SMOKE_KILL_GRACE_S:-10}"

# Parse host/port out of the Twirp URL for the readiness poll.
HOST_PORT="${TWIRP_HOST#http://}"
HOST_PORT="${HOST_PORT#https://}"
HOST_ONLY="${HOST_PORT%%:*}"
PORT_ONLY="${HOST_PORT##*:}"

# Server env: mirror cmd/run-dev.sh's resolution so the worktree's own code and
# config are what gets tested. .env must live at $TRADING_PROJECT_DIR/.env.
TRADING_PROJECT_DIR="${TRADING_PROJECT_DIR:-$REPO_ROOT}"
OPTIONS_CONFIG_PATH="${OPTIONS_CONFIG_PATH:-$REPO_ROOT/src/go/${OPTIONS_CONFIG_FILE:-options-config.yaml}}"

SERVER_LOG="$(mktemp "${TMPDIR:-/tmp}/e2e-smoke-server.XXXXXX.log")"
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
            echo "[smoke] server group $SERVER_PGID did not exit after ${KILL_GRACE_S}s; sending SIGKILL" >&2
            kill -KILL "-$SERVER_PGID" 2>/dev/null || true
        fi
        echo "[smoke] server group $SERVER_PGID torn down" >&2
    fi
    rm -f "$SERVER_LOG" 2>/dev/null || true
}
trap teardown EXIT INT TERM

# --- Fail fast if something else already holds Twirp 5051 --------------------
# We must never adopt (or later kill) a server we did not start.
if nc -z "$HOST_ONLY" "$PORT_ONLY" 2>/dev/null; then
    echo "ERROR: something is already listening on ${HOST_ONLY}:${PORT_ONLY}." >&2
    echo "       Refusing to boot a second server or adopt one this script did not start." >&2
    echo "       Stop the other server (or point E2E_SMOKE_TWIRP_HOST elsewhere) and retry." >&2
    exit 1
fi

if [ ! -f "$TRADING_PROJECT_DIR/.env" ]; then
    echo "ERROR: no .env found at ${TRADING_PROJECT_DIR}/.env (required to boot the server)." >&2
    echo "       Set TRADING_PROJECT_DIR to a repo root that has a .env, or copy one in." >&2
    exit 1
fi

# --- Boot the Go server (dev mode) in its own session/process group ----------
echo "[smoke] booting Go server from ${CMD_DIR} (GO_ENV=development), log: ${SERVER_LOG}" >&2
GO_ENV=development \
TRADING_PROJECT_DIR="$TRADING_PROJECT_DIR" \
OPTIONS_CONFIG_PATH="$OPTIONS_CONFIG_PATH" \
OTEL_SDK_DISABLED=true \
    setsid sh -c "cd '$CMD_DIR' && exec go run ./main.go" >"$SERVER_LOG" 2>&1 &
SERVER_PGID="$!"   # setsid makes this pid the leader of a new process group
echo "[smoke] server process group: ${SERVER_PGID}" >&2

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
echo "[smoke] Twirp ready on ${HOST_ONLY}:${PORT_ONLY} after ${i}s" >&2

# --- Run the pytest smoke module; capture its exit code ----------------------
# Do not let a nonzero pytest exit trip `set -e` before we record it.
set +e
OTEL_SDK_DISABLED=true \
E2E_SMOKE_HARNESS=1 \
E2E_SMOKE_TWIRP_HOST="$TWIRP_HOST" \
    "$PYTHON" -m pytest "$SCRIPT_DIR/tests/test_e2e_smoke.py" -v -s
TEST_RC=$?
set -e

if [ "$TEST_RC" -eq 0 ]; then
    echo "[smoke] PASSED — server booted, order filled, teardown clean." >&2
else
    echo "[smoke] FAILED — pytest exit code ${TEST_RC}." >&2
fi

# teardown runs via trap; propagate the pytest exit code.
exit "$TEST_RC"
