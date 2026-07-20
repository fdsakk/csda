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

# Bun is the repository's only frontend package manager.
minimum_bun_version="1.3.0"
if ! command -v bun >/dev/null 2>&1; then
  echo "error: Bun ${minimum_bun_version} or newer is required to build the web UI" >&2
  exit 1
fi

bun_version="$(bun --version)"
if [[ ! "$bun_version" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  echo "error: unable to parse Bun version '$bun_version'" >&2
  exit 1
fi

bun_major=$((10#${BASH_REMATCH[1]}))
bun_minor=$((10#${BASH_REMATCH[2]}))
if ((bun_major < 1 || (bun_major == 1 && bun_minor < 3))); then
  echo "error: Bun ${minimum_bun_version} or newer is required (found ${bun_version})" >&2
  exit 1
fi

echo "Building web UI with Bun..."
(cd web && bun install --frozen-lockfile && bun run build)

echo "Starting web server..."
exec go run ./cmd/cli web "$@"
