#!/bin/bash

export GO_ENV=development

# Resolve the repo root (parent of this script's directory)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Override config path so it resolves correctly in worktrees
export OPTIONS_CONFIG_PATH="${REPO_ROOT}/src/go/${OPTIONS_CONFIG_FILE:-options-config.yaml}"

go run ./main.go "$@"
