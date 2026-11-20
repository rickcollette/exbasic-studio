#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="${1:?usage: run-linux.sh OUTPUT_DIRECTORY}"
OUT="$(mkdir -p "$OUT" && cd "$OUT" && pwd)"

command -v docker >/dev/null 2>&1 || {
  echo "Docker is required on the Linux SSH host." >&2
  exit 1
}
docker info >/dev/null

WORK="$(mktemp -d "${TMPDIR:-/tmp}/exbasic-linux-lanes.XXXXXX")"
cleanup() {
  case "$WORK" in "${TMPDIR:-/tmp}"/exbasic-linux-lanes.*) rm -rf -- "$WORK" ;; esac
}
trap cleanup EXIT

for lane in compat current; do
  echo "== Linux $lane lane =="
  mkdir -p "$WORK/$lane"
  # Each lane gets a private copy because the compatibility build adjusts its
  # package dependency metadata and both tasks generate intermediate files.
  (cd "$ROOT" && tar \
    --exclude='./bin' \
    --exclude='./DIST' \
    --exclude='./build/stage' \
    --exclude='./build/toolchain' \
    --exclude='./frontend/dist' \
    --exclude='./frontend/node_modules' \
    -cf - .) | (cd "$WORK/$lane" && tar -xf -)

  image="exbasic-studio-linux-$lane:local"
  docker build \
    --file "$ROOT/packaging/remote/Dockerfile.linux-$lane" \
    --tag "$image" \
    "$ROOT/packaging/remote"
  docker run --rm \
    --user "$(id -u):$(id -g)" \
    --env HOME=/tmp/exbasic-build-home \
    --volume "$WORK/$lane:/workspace" \
    --volume "$OUT:/out" \
    "$image"
done

echo "Linux artifacts: $OUT"
