#!/usr/bin/env bash
# Build the web UI and start the CS Demo Analyzer web server.
# Any extra args are forwarded to `csda web` (e.g. ./run.sh --addr=127.0.0.1:9000).
set -euo pipefail

cd "$(dirname "$0")"

# Load local settings (e.g. CSDA_AUTH_USER / CSDA_AUTH_PASSWORD), if present.
if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi

# Bun is the repository's only frontend package manager. Keep its version in
# sync with web/package.json and the Docker/CI configuration.
if ! command -v bun >/dev/null 2>&1; then
  echo "error: Bun 1.3.0 is required to build the web UI" >&2
  exit 1
fi
if [ "$(bun --version)" != "1.3.0" ]; then
  echo "error: Bun 1.3.0 is required (found $(bun --version))" >&2
  exit 1
fi

echo "Building web UI with Bun..."
(cd web && bun install --frozen-lockfile && bun run build)

echo "Starting web server..."
exec go run ./cmd/cli web "$@"
