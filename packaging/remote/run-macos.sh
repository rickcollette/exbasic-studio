#!/usr/bin/env bash
# Runs on the configured Apple Silicon SSH builder.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="${1:?usage: run-macos.sh OUTPUT_DIRECTORY}"
OUT="$(mkdir -p "$OUT" && cd "$OUT" && pwd)"
export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/local/go/bin:$HOME/go/bin:$PATH"

# The release host downloads pinned Go tools from the public module proxy.
# Retry transient TLS/network failures, but keep a firm upper bound so a real
# configuration problem still fails the build promptly.
retry_network() {
  local attempt=1
  local max_attempts=4
  local delay=5
  while true; do
    if "$@"; then
      return 0
    fi
    if [ "$attempt" -ge "$max_attempts" ]; then
      echo "network command failed after $attempt attempts: $1" >&2
      return 1
    fi
    echo "network command failed; retrying in ${delay}s ($attempt/$max_attempts)" >&2
    sleep "$delay"
    attempt=$((attempt + 1))
    delay=$((delay * 2))
  done
}

test "$(uname -s)" = Darwin || { echo "the MAC SSH entry must reach macOS" >&2; exit 1; }
test "$(uname -m)" = arm64 || {
  echo "The macOS builder must be Apple Silicon so it can test arm64 natively and amd64 through Rosetta." >&2
  exit 1
}
arch -x86_64 /usr/bin/true >/dev/null 2>&1 || {
  echo "Rosetta 2 is required for the Intel self-test (softwareupdate --install-rosetta)." >&2
  exit 1
}

for command in go node npm python3 xcrun codesign hdiutil ditto lipo git make; do
  command -v "$command" >/dev/null 2>&1 || { echo "missing macOS build prerequisite: $command" >&2; exit 1; }
done

VERSION="$(tr -d '\r\n' < "$ROOT/VERSION")"
BUILD_NUMBER="$(tr -d '\r\n' < "$ROOT/BUILD_NUMBER")"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/exbasic-macos-arches.XXXXXX")"
cleanup() {
  case "$WORK" in "${TMPDIR:-/tmp}"/exbasic-macos-arches.*) rm -rf -- "$WORK" ;; esac
}
trap cleanup EXIT

TOOL_CACHE="${EXBASIC_RELEASE_TOOL_CACHE:-$HOME/.cache/exbasic-studio-release/bin}"
mkdir -p "$TOOL_CACHE"
export PATH="$TOOL_CACHE:$PATH"

install_go_tool() {
  local command_name="$1"
  local module="$2"
  if command -v "$command_name" >/dev/null 2>&1; then
    return 0
  fi

  # A previous release build leaves the module source in Go's module cache.
  # Build from it without touching the network, then retain the resulting
  # executable for every later release job on this host.
  echo "Installing $command_name from the local Go module cache"
  if env GOPROXY=off GOSUMDB=off GOBIN="$TOOL_CACHE" go install "$module"; then
    return 0
  fi

  echo "$command_name is not fully cached; fetching its pinned tag directly" >&2
  retry_network env GOPROXY=direct GOBIN="$TOOL_CACHE" go install "$module"
}

install_go_tool task github.com/go-task/task/v3/cmd/task@v3.40.1
install_go_tool wails3 github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0

build_arch() {
  ARCH="$1"
  echo "== macOS $ARCH =="
  SRC="$WORK/$ARCH/source"
  mkdir -p "$SRC" "$WORK/$ARCH/wrappers"
  # remote_build.py has already removed generated outputs from ROOT. Avoid
  # bsdtar exclusions here: on macOS a pattern such as './bin' also matches
  # nested directories and would remove the vendored MADS host binaries.
  (cd "$ROOT" && tar -cf - .) | (cd "$SRC" && tar -xf -)

  CLANG_ARCH="$ARCH"
  [ "$ARCH" = amd64 ] && CLANG_ARCH=x86_64
  CC_WRAPPER="$WORK/$ARCH/wrappers/clang"
  CXX_WRAPPER="$WORK/$ARCH/wrappers/clang++"
  cat > "$CC_WRAPPER" <<EOF
#!/bin/sh
exec xcrun --sdk macosx clang -arch $CLANG_ARCH "\$@"
EOF
  cat > "$CXX_WRAPPER" <<EOF
#!/bin/sh
exec xcrun --sdk macosx clang++ -arch $CLANG_ARCH "\$@"
EOF
  chmod +x "$CC_WRAPPER" "$CXX_WRAPPER"

  (
    cd "$SRC"
    # The source archive originates on Windows, so restore the execute bit
    # required when the native toolchain invokes this generated script.
    chmod +x \
      third_party/ex800/ex800-1.0.0/compile \
      third_party/ex800/ex800-1.0.0/config.guess \
      third_party/ex800/ex800-1.0.0/config.sub \
      third_party/ex800/ex800-1.0.0/configure \
      third_party/ex800/ex800-1.0.0/depcomp \
      third_party/ex800/ex800-1.0.0/install-sh \
      third_party/ex800/ex800-1.0.0/missing
    python3 - <<'PY'
from pathlib import Path

path = Path("build/tools/build-toolchain.sh")
text = path.read_text()
text = text.replace(
    'CONFIG_ARGS="--disable-maintainer-mode ',
    'CONFIG_ARGS="--disable-maintainer-mode --disable-dependency-tracking ',
)
path.write_text(text)
PY
    export GOOS=darwin GOARCH="$ARCH" CGO_ENABLED=1 CC="$CC_WRAPPER" CXX="$CXX_WRAPPER"
    export MACOSX_DEPLOYMENT_TARGET=12.0
    export CGO_CFLAGS="-mmacosx-version-min=12.0"
    export CGO_LDFLAGS="-mmacosx-version-min=12.0"
    task version:check
    task toolchain ARCH="$ARCH"
    task darwin:build ARCH="$ARCH" OUTPUT=bin/exbasic-studio
    task darwin:create:app:bundle ARCH="$ARCH" TOOLS_DIR="build/stage/darwin-$ARCH/tools"

    BINARY="bin/exbasic-studio.app/Contents/MacOS/exbasic-studio"
    test -x "$BINARY"
    lipo -archs "$BINARY" | grep -q "$CLANG_ARCH"
    "$BINARY" --self-test "$WORK/$ARCH/self-test.txt"
    cat "$WORK/$ARCH/self-test.txt"
    codesign --verify --deep --strict bin/exbasic-studio.app

    ZIP="exbasic-studio-$VERSION-macos-$ARCH.app.zip"
    ditto -c -k --sequesterRsrc --keepParent bin/exbasic-studio.app "$OUT/$ZIP"

    rm -f bin/*.dmg
    task darwin:create:dmg
    DMG="$(find bin -maxdepth 1 -type f -name '*.dmg' -print -quit)"
    test -s "$DMG" || { echo "DMG packaging did not produce a file" >&2; exit 1; }
    mv "$DMG" "$OUT/exbasic-studio-$VERSION-macos-$ARCH.dmg"
  )
}

PIDS=()
build_arch amd64 & PIDS+=("$!")
build_arch arm64 & PIDS+=("$!")
STATUS=0
for PID in "${PIDS[@]}"; do
  wait "$PID" || STATUS=1
done
test "$STATUS" -eq 0 || { echo "one or more macOS architecture builds failed" >&2; exit 1; }

python3 - "$VERSION" "$BUILD_NUMBER" "$OUT/exbasic-studio-$VERSION-macos-build-manifest.json" <<'PY'
import json, platform, subprocess, sys
version, build, output = sys.argv[1:]
sdk = subprocess.check_output(["xcrun", "--sdk", "macosx", "--show-sdk-version"], text=True).strip()
document = {
    "schemaVersion": 1,
    "applicationVersion": version,
    "buildNumber": int(build),
    "platform": "macos",
    "architectures": ["amd64", "arm64"],
    "minimumMacOS": "12.0",
    "sdkVersion": sdk,
    "builderMacOS": platform.mac_ver()[0],
    "signing": "ad-hoc",
    "notarized": False,
}
with open(output, "w", encoding="utf-8") as stream:
    json.dump(document, stream, indent=2, sort_keys=True)
    stream.write("\n")
PY

echo "macOS Intel and Apple Silicon artifacts: $OUT"
