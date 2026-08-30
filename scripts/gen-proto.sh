#!/usr/bin/env sh
set -eu

command -v docker >/dev/null || {
  echo "missing docker" >&2
  exit 1
}

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
exec docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp \
  -v "$root:/workspace" \
  -w /workspace \
  bufbuild/buf@sha256:edf71e8832862ae1b1cbbcea245231383fb66ad1b19dbb40fca74f02482b21f1 generate
