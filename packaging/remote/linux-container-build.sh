#!/usr/bin/env bash
# Runs inside one of the pinned Linux release containers.
set -euo pipefail

LANE="${1:?lane is required}"
case "$LANE" in compat|current) ;; *) echo "unknown Linux lane: $LANE" >&2; exit 2 ;; esac
test -d /out || { echo "/out is not mounted" >&2; exit 2; }

VERSION="$(tr -d '\r\n' < VERSION)"
BUILD_NUMBER="$(tr -d '\r\n' < BUILD_NUMBER)"
ARCH="$(go env GOARCH)"
test "$ARCH" = amd64 || { echo "the remote Linux release lane currently requires amd64" >&2; exit 2; }

export GIT_COMMITTER_NAME="exBASIC Studio Build"
export GIT_COMMITTER_EMAIL="build@exbasic.invalid"
export EXBASIC_STUDIO_REQUIRE_TOOLS=1
export APPIMAGE_EXTRACT_AND_RUN=1

EXTRA_TAGS=""
GUI_ABI="gtk4-webkitgtk6"
if [ "$LANE" = compat ]; then
  EXTRA_TAGS="gtk3"
  GUI_ABI="gtk3-webkit2gtk4.1"
fi

python3 - "$LANE" "$ARCH" <<'PY'
from pathlib import Path
import sys

lane, arch = sys.argv[1:]
path = Path("build/linux/nfpm/nfpm.yaml")
text = path.read_text()
text = text.replace("${GOARCH}", arch)
if lane == "compat":
    text = text.replace("libgtk-4-1", "libgtk-3-0")
    text = text.replace("libwebkitgtk-6.0-4", "libwebkit2gtk-4.1-0")
    text = text.replace("      - gtk4\n      - webkitgtk6.0", "      - gtk3\n      - webkit2gtk4.1")
path.write_text(text)
PY

rm -rf bin build/stage build/toolchain frontend/dist
mkdir -p bin
# Source snapshots are created on Windows, whose tar metadata cannot represent
# Unix execute bits. Restore the vendored configure script used directly by
# the native toolchain build.
chmod +x third_party/ex800/ex800-1.0.0/configure
task version:check
task toolchain
task linux:package EXTRA_TAGS="$EXTRA_TAGS"

# Wails' built-in AppImage generator only copies the application binary. Add
# the separately licensed tool bundle to its completed AppDir and seal it
# again with the same linuxdeploy binary.
APPIMAGE_BUILD="$PWD/build/linux/appimage/build"
APPDIR="$(find "$APPIMAGE_BUILD" -maxdepth 1 -type d -name '*.AppDir' -print -quit)"
LINUXDEPLOY="$(find "$APPIMAGE_BUILD" -maxdepth 1 -type f -name 'linuxdeploy-*.AppImage' -print -quit)"
test -d "$APPDIR" && test -x "$LINUXDEPLOY"
rm -rf "$APPDIR/usr/lib/exbasic-studio/tools"
mkdir -p "$APPDIR/usr/lib/exbasic-studio"
cp -R "build/stage/linux-$ARCH/tools" "$APPDIR/usr/lib/exbasic-studio/tools"
REPACKED_NAME="exbasic-studio-with-tools-x86_64.AppImage"
rm -f "$APPIMAGE_BUILD/$REPACKED_NAME"
(
  cd "$APPIMAGE_BUILD"
  env APPIMAGE_EXTRACT_AND_RUN=1 NO_STRIP=1 OUTPUT="$REPACKED_NAME" \
    "$LINUXDEPLOY" --appimage-extract-and-run --appdir "$APPDIR" --output appimage
)
test -s "$APPIMAGE_BUILD/$REPACKED_NAME"
rm -f bin/*.AppImage
mv "$APPIMAGE_BUILD/$REPACKED_NAME" bin/exbasic-studio-x86_64.AppImage

BINARY="bin/exbasic-studio"
APPIMAGE="$(find bin -maxdepth 1 -type f -name '*.AppImage' -print -quit)"
DEB="$(find bin -maxdepth 1 -type f -name '*.deb' -print -quit)"
RPM="$(find bin -maxdepth 1 -type f -name '*.rpm' -print -quit)"
for required in "$BINARY" "$APPIMAGE" "$DEB" "$RPM"; do
  test -s "$required" || { echo "missing release output: $required" >&2; exit 1; }
done
APPIMAGE="$(readlink -f "$APPIMAGE")"

# A raw binary uses bin/tools, while deb/rpm and AppImage have their own
# platform-specific package layouts.
rm -rf bin/tools
cp -R "build/stage/linux-$ARCH/tools" bin/tools
REPORT="/tmp/exbasic-selftest-$LANE.txt"
"$BINARY" --self-test "$REPORT"
cat "$REPORT"

APPIMAGE_TEST_DIR="/tmp/exbasic-appimage-$LANE"
rm -rf "$APPIMAGE_TEST_DIR"
mkdir -p "$APPIMAGE_TEST_DIR"
(cd "$APPIMAGE_TEST_DIR" && env -u APPIMAGE_EXTRACT_AND_RUN "$APPIMAGE" --appimage-extract >/dev/null)
APPIMAGE_ROOT="$APPIMAGE_TEST_DIR/squashfs-root"
test -d "$APPIMAGE_ROOT/usr/lib/exbasic-studio/tools"
EXBASIC_STUDIO_TOOLS="$APPIMAGE_ROOT/usr/lib/exbasic-studio/tools" \
  "$APPIMAGE_ROOT/usr/bin/exbasic-studio" --self-test "$REPORT"
cat "$REPORT"

GLIBC_RUNTIME="$(getconf GNU_LIBC_VERSION | awk '{print $2}')"
GLIBC_REQUIRED="$(objdump -T "$BINARY" | sed -n 's/.*(GLIBC_\([0-9.]*\)).*/\1/p' | sort -Vu | tail -n 1)"
PREFIX="exbasic-studio-$VERSION-linux-$ARCH-$LANE"
install -m 0755 "$APPIMAGE" "/out/$PREFIX.AppImage"
install -m 0644 "$DEB" "/out/$PREFIX.deb"
install -m 0644 "$RPM" "/out/$PREFIX.rpm"

python3 - "$VERSION" "$BUILD_NUMBER" "$LANE" "$ARCH" "$GUI_ABI" "$GLIBC_RUNTIME" "$GLIBC_REQUIRED" "/out/$PREFIX-build-manifest.json" <<'PY'
import json, platform, sys
version, build, lane, arch, gui, runtime, required, output = sys.argv[1:]
document = {
    "schemaVersion": 1,
    "applicationVersion": version,
    "buildNumber": int(build),
    "platform": "linux",
    "architecture": arch,
    "lane": lane,
    "guiAbi": gui,
    "buildGlibc": runtime,
    "maximumRequiredGlibcSymbol": required,
    "supportedDistributions": (
        ["Ubuntu 22.04+", "Debian 12/current", "Fedora current"]
        if lane == "compat" else
        ["Ubuntu 24.04+", "Debian current", "Fedora current"]
    ),
    "unsupportedDistributions": ["Ubuntu 20.04"],
    "kernel": platform.release(),
}
with open(output, "w", encoding="utf-8") as stream:
    json.dump(document, stream, indent=2, sort_keys=True)
    stream.write("\n")
PY

echo "Built $PREFIX (glibc runtime $GLIBC_RUNTIME, maximum required symbol $GLIBC_REQUIRED)"
