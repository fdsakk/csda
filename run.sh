#!/usr/bin/env bash
# Build the web UI and start the CS Demo Analyzer web server.
# Any extra args are forwarded to `csda web` (e.g. ./run.sh --addr=127.0.0.1:9000).
set -euo pipefail

cd "$(dirname "$0")"

# Pick a package manager for the frontend build.
if command -v bun >/dev/null 2>&1; then
  PM=bun
elif command -v npm >/dev/null 2>&1; then
  PM=npm
else
  echo "error: need bun or npm to build the web UI" >&2
  exit 1
fi

echo "Building web UI with $PM..."
(cd web && "$PM" install && "$PM" run build)

echo "Starting web server..."
exec go run ./cmd/cli web "$@"
