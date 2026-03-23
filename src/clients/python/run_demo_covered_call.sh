#!/usr/bin/env bash
# Run the covered call strategy demo using the grodt conda environment.
# Usage: ./run_demo_covered_call.sh [--symbol AAPL] [--start 2024-06-01] [--end 2024-12-31] ...
#
# All arguments are forwarded to demo_covered_call.py.
# Requires the Go trading server to be running (default: http://127.0.0.1:5051).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PYTHON="/Users/jamal/miniconda3/envs/grodt/bin/python"
TWIRP_HOST="http://127.0.0.1:5051"

# Extract --twirp-host from args if provided
prev=""
for arg in "$@"; do
    if [[ "$prev" == "--twirp-host" ]]; then TWIRP_HOST="$arg"; fi
    prev="$arg"
done

# Parse host:port from URL
HOST_PORT="${TWIRP_HOST#http://}"
HOST_PORT="${HOST_PORT#https://}"
HOST="${HOST_PORT%%:*}"
PORT="${HOST_PORT##*:}"

# Check that the server is reachable
if ! nc -z "$HOST" "$PORT" 2>/dev/null; then
    echo "ERROR: Go trading server is not reachable at ${TWIRP_HOST}" >&2
    echo "" >&2
    echo "Start the server first, e.g.:" >&2
    echo "  cd \$(git rev-parse --show-toplevel) && go run ./cmd/main.go" >&2
    exit 1
fi

echo "Server is reachable at ${TWIRP_HOST}"
cd "$SCRIPT_DIR"
exec "$PYTHON" demo_covered_call.py "$@"
