#!/usr/bin/env bash
# Builds the separately licensed tools that ship with exBASIC Studio from the
# pinned sources in third_party/ and stages them for packaging:
#
#   exBASIC compiler distribution   third_party/exbasic/source      (GPL-2.0+)
#   Atari800 core (libatari800)     third_party/atari800/atari800-* (GPL-2.0+)
#   atari800-studio helper          third_party/atari800/studio-helper
#   mkatr-go                        cmd/mkatr-go, third_party/mkatrgo
#   ExtendedOS source/license       third_party/exoperatingsystem (MIT)
#   EOS cartridge                   third_party/EOS.car
#
# Output: build/stage/<os>-<arch>/tools, with each tool's license beside it.
# Intermediate files go to build/toolchain/<os>-<arch>; nothing is written
# into the vendored source trees except exBASIC's own (ignored) build/ folder.
#
# Requirements: bash, make, a C and C++ compiler, and Go. On Windows run it
# from Git Bash with MinGW-w64 (gcc, g++, make) on PATH.
#
# Usage: build/tools/build-toolchain.sh [--clean]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
PLATFORM="$GOOS-$GOARCH"
EXE=""
[ "$GOOS" = "windows" ] && EXE=".exe"
JOBS="${JOBS:-4}"
CC="${CC:-gcc}"
command -v "$CC" >/dev/null 2>&1 || CC=cc

WORK="build/toolchain/$PLATFORM"
STAGE="build/stage/$PLATFORM/tools"
ATARI_SRC="third_party/atari800/atari800-7.2.1"
EXBASIC_SRC="third_party/exbasic/source"

if [ "${1:-}" = "--clean" ]; then
	rm -rf "$WORK" "build/stage/$PLATFORM"
	make -C "$EXBASIC_SRC" distclean >/dev/null 2>&1 || true
fi

say() { printf '\n== %s\n' "$*"; }

# ---- exBASIC -----------------------------------------------------------
say "exBASIC compiler distribution"
make -C "$EXBASIC_SRC" -j"$JOBS" all

# ---- Atari800 core -----------------------------------------------------
say "Atari800 core (libatari800, with code-breakpoint support)"
# Git does not keep modification times, so the generated autotools files can
# look older than their inputs and make would try to regenerate them. Order
# the timestamps the way a release tarball has them. This is done only when
# configuring: touching them on every run would make an existing build tree
# look stale and send make back into config.status.
CORE="$WORK/atari800"
STAMP="$CORE/.configured"
CONFIG_ARGS="--disable-maintainer-mode --target=libatari800 --enable-riodevice --enable-rnetwork --enable-monitorbreak"
if [ ! -f "$STAMP" ] || [ "$(cat "$STAMP")" != "$CONFIG_ARGS" ]; then
	touch "$ATARI_SRC/aclocal.m4"
	sleep 1
	touch "$ATARI_SRC/configure"
	sleep 1
	find "$ATARI_SRC" \( -name 'Makefile.in' -o -name 'config.h.in' \) -exec touch {} +
	rm -rf "$CORE"
	mkdir -p "$CORE"
	# A relative source path keeps the build working with native Windows make.
	(cd "$CORE" && "../../../../$ATARI_SRC/configure" $CONFIG_ARGS)
	printf '%s' "$CONFIG_ARGS" > "$STAMP"
fi
make -C "$CORE" -j"$JOBS" CFLAGS="${CFLAGS:--O2} -Wno-error"
test -f "$CORE/src/libatari800.a"

# ---- Stage ---------------------------------------------------------------
say "Staging to $STAGE"
rm -rf "$STAGE"
mkdir -p "$STAGE/exbasic" "$STAGE/licenses"
cp -R "$EXBASIC_SRC/build/compiler/." "$STAGE/exbasic/"

HELPER_FLAGS="-O2"
HELPER_LIBS="-lm"
if [ "$GOOS" = "windows" ]; then
	HELPER_FLAGS="-O2 -static"
	HELPER_LIBS="-lm -lws2_32"
fi
# shellcheck disable=SC2086
"$CC" $HELPER_FLAGS ${CFLAGS:-} -Wall -I "$CORE/src" -I "$ATARI_SRC/src" \
	third_party/atari800/studio-helper/main.c "$CORE/src/libatari800.a" \
	-o "$STAGE/atari800-studio$EXE" $HELPER_LIBS

CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-w -s" -o "$STAGE/mkatr-go$EXE" ./cmd/mkatr-go
cp third_party/EOS.car "$STAGE/EOS.car"

# Every separately licensed tool ships with its license and notice.
cp "$ATARI_SRC/COPYING" "$STAGE/licenses/atari800-COPYING.txt"
cp third_party/atari800/NOTICE.md "$STAGE/licenses/atari800-NOTICE.md"
cp "$EXBASIC_SRC/LICENSE" "$STAGE/licenses/exbasic-LICENSE.txt"
cp third_party/exbasic/NOTICE.md "$STAGE/licenses/exbasic-NOTICE.md"
cp third_party/mkatrgo/LICENSE "$STAGE/licenses/mkatr-go-LICENSE.txt"
cp third_party/mkatrgo/NOTICE.md "$STAGE/licenses/mkatr-go-NOTICE.md"
cp third_party/exoperatingsystem/LICENSE "$STAGE/licenses/EOS-LICENSE.txt"

# ---- Verify ----------------------------------------------------------------
say "Verifying staged tools"
"$STAGE/exbasic/exbasic$EXE" --capabilities=json
for tool in "exbasic/exbasic$EXE" "exbasic/ca65$EXE" "exbasic/ld65$EXE" "atari800-studio$EXE" "mkatr-go$EXE" "EOS.car"; do
	test -s "$STAGE/$tool" || { echo "missing staged tool: $tool" >&2; exit 1; }
done
echo "Toolchain staged in $ROOT/$STAGE"
