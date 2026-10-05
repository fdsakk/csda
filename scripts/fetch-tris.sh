#!/usr/bin/env bash
# Install the map geometry (.tri files) used for visibility checks into ./tris
# and verify it against the pinned sums in scripts/tris.sha256.
#
#   scripts/fetch-tris.sh                  download with Awpy (Python 3.11+, network)
#   scripts/fetch-tris.sh path/to/tris.zip use a local archive, no network
#
# The pinned sums make every checkout analyze demos against identical geometry.
set -euo pipefail

AWPY_VERSION="2.0.2"
# Maps the thesis scope relies on (see docs/SCOPE.md); the script fails if one is missing.
REQUIRED_MAPS=(de_mirage de_nuke de_anubis)

cd "$(dirname "$0")/.."
root="$PWD"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

if [ $# -ge 1 ]; then
  archive="$(realpath "$1")"
  if [ ! -f "$archive" ]; then
    echo "error: $1 not found" >&2
    exit 1
  fi
  mkdir "$work/tris"
  unzip -q "$archive" -d "$work/tris"
  source_dir="$work/tris"
else
  if ! command -v python3 >/dev/null 2>&1; then
    echo "error: python3 is required to download the map geometry (or pass a local tris.zip)" >&2
    exit 1
  fi
  python3 -m venv "$work/venv"
  "$work/venv/bin/pip" install --quiet --disable-pip-version-check "awpy==${AWPY_VERSION}"
  # awpy stores its data under $HOME/.awpy; point HOME at the temp dir to keep the user's home clean.
  HOME="$work" "$work/venv/bin/awpy" get tris
  source_dir="$work/.awpy/tris"
fi

# Verify before touching ./tris so a bad download never replaces good files.
(cd "$source_dir" && sha256sum --quiet -c "$root/scripts/tris.sha256") || {
  echo "error: geometry does not match scripts/tris.sha256" >&2
  exit 1
}
for map in "${REQUIRED_MAPS[@]}"; do
  if [ ! -s "$source_dir/${map}.tri" ]; then
    echo "error: missing or empty geometry ${map}.tri" >&2
    exit 1
  fi
done

mkdir -p tris
cp "$source_dir"/*.tri tris/
echo "Geometry for $(ls tris/*.tri | wc -l) maps installed in tris/ and verified against scripts/tris.sha256"
