#!/usr/bin/env bash
set -euo pipefail

usage() {
	printf 'Usage: %s {linux-x86_64|windows-x86_64|macos-x86_64|macos-arm64}\n' "$0" >&2
	exit 2
}

[[ $# -eq 1 ]] || usage
target=$1

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
version=$(tr -d '\r\n' < "$repo_root/VERSION")
source_dir="$repo_root/ex800-$version"
build_root=${BUILD_ROOT:-"$repo_root/build/out"}
dist_root=${DIST_ROOT:-"$repo_root/dist"}
target_build="$build_root/$target"
app_build="$target_build/app"
core_build="$target_build/core"
stage="$target_build/ex800-$version-$target"

[[ -x "$source_dir/configure" ]] || {
	printf 'Missing configure script: %s\n' "$source_dir/configure" >&2
	exit 1
}

case "$target" in
	linux-x86_64)
		[[ $(uname -s) == Linux && $(uname -m) == x86_64 ]] || {
			printf '%s must be built natively on Linux x86_64.\n' "$target" >&2
			exit 1
		}
		archive_type=tar
		exe_suffix=
		runtime_note='Requires SDL2, libpng and zlib runtime libraries from the Linux distribution.'
		;;
	windows-x86_64)
		case "${MSYSTEM:-}" in
			UCRT64) ;;
			*) printf '%s must be built in an MSYS2 UCRT64 shell.\n' "$target" >&2; exit 1 ;;
		esac
		archive_type=zip
		exe_suffix=.exe
		runtime_note='Required non-system UCRT64 DLLs are included in this archive.'
		;;
	macos-x86_64)
		[[ $(uname -s) == Darwin ]] || {
			printf '%s must be built on macOS.\n' "$target" >&2
			exit 1
		}
		if [[ $(uname -m) != x86_64 ]] && ! arch -x86_64 /usr/bin/true 2>/dev/null; then
			printf '%s requires an Intel Mac or Rosetta 2.\n' "$target" >&2
			exit 1
		fi
		target_arch=x86_64
		archive_type=tar
		exe_suffix=
		runtime_note='Homebrew runtime libraries are bundled; the app has an ad-hoc signature.'
		;;
	macos-arm64)
		[[ $(uname -s) == Darwin && $(uname -m) == arm64 ]] || {
			printf '%s must be built natively on an Apple Silicon Mac.\n' "$target" >&2
			exit 1
		}
		target_arch=arm64
		archive_type=tar
		exe_suffix=
		runtime_note='Homebrew runtime libraries are bundled; the app has an ad-hoc signature.'
		;;
	*) usage ;;
esac

if [[ $target == macos-* && -n ${MACOS_CROSS_PREFIX:-} ]]; then
	export PATH="$MACOS_CROSS_PREFIX/bin:$PATH"
	export CPPFLAGS="-I$MACOS_CROSS_PREFIX/include ${CPPFLAGS:-}"
	export LDFLAGS="-L$MACOS_CROSS_PREFIX/lib ${LDFLAGS:-}"
	export PKG_CONFIG_PATH="$MACOS_CROSS_PREFIX/lib/pkgconfig:${PKG_CONFIG_PATH:-}"
elif [[ $target == macos-* ]] && command -v brew >/dev/null 2>&1; then
	sdl_prefix=$(brew --prefix sdl2)
	png_prefix=$(brew --prefix libpng)
	export PATH="$sdl_prefix/bin:$PATH"
	export CPPFLAGS="-I$png_prefix/include ${CPPFLAGS:-}"
	export LDFLAGS="-L$png_prefix/lib ${LDFLAGS:-}"
	export PKG_CONFIG_PATH="$sdl_prefix/lib/pkgconfig:$png_prefix/lib/pkgconfig:${PKG_CONFIG_PATH:-}"
fi

if [[ $target == macos-* ]]; then
	export MACOSX_DEPLOYMENT_TARGET=${MACOSX_DEPLOYMENT_TARGET:-11.0}
	export CFLAGS="-arch $target_arch ${CFLAGS:-}"
	export LDFLAGS="-arch $target_arch ${LDFLAGS:-}"
fi

jobs=${JOBS:-}
if [[ -z $jobs ]]; then
	if command -v nproc >/dev/null 2>&1; then
		jobs=$(nproc)
	else
		jobs=$(sysctl -n hw.ncpu 2>/dev/null || printf '2')
	fi
fi

mkdir -p "$app_build" "$core_build" "$dist_root"
find "$stage" -depth -delete 2>/dev/null || true
mkdir -p "$stage"

common_configure=(
	--disable-download
	--without-readline
	--disable-maintainer-mode
)

(
	cd "$app_build"
	"$source_dir/configure" "${common_configure[@]}" \
		--with-video=sdl2 --with-sound=sdl2
	make -j"$jobs"
)

(
	cd "$core_build"
	"$source_dir/configure" "${common_configure[@]}" \
		--target=libatari800 --enable-monitorbreak
	make -j"$jobs"
)

helper_libs=(-lm -lz)
if [[ $target == windows-* ]]; then
	helper_libs+=(-lws2_32)
else
	helper_libs+=(-lpthread)
fi

${CC:-cc} ${CPPFLAGS:-} ${CFLAGS:-} -std=c99 -O2 -Wall -Wextra -Werror \
	-I"$core_build/src" -I"$source_dir/src" \
	"$repo_root/studio-helper/main.c" "$core_build/src/libatari800.a" \
	${LDFLAGS:-} "${helper_libs[@]}" -o "$core_build/studio-helper$exe_suffix"

# The inherited command-line frontend uses a non-zero informational exit code
# for -v, so validate its text rather than its status.
version_output=$("$app_build/src/ex800$exe_suffix" -v || true)
[[ $version_output == "ex800, Version $version" ]] || {
	printf 'Unexpected version output: %s\n' "$version_output" >&2
	exit 1
}
"$core_build/studio-helper$exe_suffix" </dev/null >/dev/null

install -m 644 "$source_dir/COPYING" "$stage/COPYING"
install -m 644 "$source_dir/README.TXT" "$stage/README.TXT"

case "$target" in
	linux-*)
		mkdir -p "$stage/bin" "$stage/share/man/man1"
		install -m 755 "$app_build/src/ex800" "$stage/bin/ex800"
		install -m 755 "$core_build/studio-helper" "$stage/bin/studio-helper"
		install -m 644 "$source_dir/src/ex800.man" "$stage/share/man/man1/ex800.1"
		;;
	windows-*)
		install -m 755 "$app_build/src/ex800.exe" "$stage/ex800.exe"
		install -m 755 "$core_build/studio-helper.exe" "$stage/studio-helper.exe"
		# Copy the complete non-system DLL closure reported by MSYS2's ldd.
		while :; do
			copied=0
			while IFS= read -r dependency; do
				[[ -n $dependency ]] || continue
				name=$(basename "$dependency")
				if [[ ! -e "$stage/$name" ]]; then
					install -m 755 "$dependency" "$stage/$name"
					copied=1
				fi
			done < <(
				for binary in "$stage"/*.exe "$stage"/*.dll; do
					[[ -e $binary ]] || continue
					ldd "$binary" 2>/dev/null | awk '{ for (i = 1; i <= NF; i++) if ($i ~ /^\/ucrt64\/bin\/.*\.[dD][lL][lL]$/) print $i }'
				done | sort -u
			)
			[[ $copied -eq 1 ]] || break
		done
		;;
	macos-*)
		command -v dylibbundler >/dev/null 2>&1 || {
			printf 'dylibbundler is required to package macOS runtime libraries.\n' >&2
			exit 1
		}
		app="$stage/ex800.app"
		mkdir -p "$app/Contents/MacOS" "$app/Contents/Frameworks" \
			"$stage/bin" "$stage/lib" "$stage/share/man/man1"
		install -m 755 "$app_build/src/ex800" "$app/Contents/MacOS/ex800"
		install -m 755 "$core_build/studio-helper" "$stage/bin/studio-helper"
		install -m 644 "$source_dir/src/ex800.man" "$stage/share/man/man1/ex800.1"
		ln -s ../ex800.app/Contents/MacOS/ex800 "$stage/bin/ex800"
		sed "s/@VERSION@/$version/g" "$repo_root/build/macos/Info.plist.in" > "$app/Contents/Info.plist"
		dylibbundler -od -b -x "$app/Contents/MacOS/ex800" \
			-d "$app/Contents/Frameworks" -p '@executable_path/../Frameworks/'
		dylibbundler -od -b -x "$stage/bin/studio-helper" \
			-d "$stage/lib" -p '@executable_path/../lib/'
		codesign --force --deep --sign - "$app"
		codesign --force --sign - "$stage/bin/studio-helper"
		;;
esac

printf '%s\n' \
	"ex800 $version" \
	"Target: $target" \
	"$runtime_note" \
	"The emulator does not include proprietary Atari OS or BASIC ROM images." \
	> "$stage/BUILD-INFO.txt"

archive_base="ex800-$version-$target"
if [[ $archive_type == zip ]]; then
	(
		cd "$target_build"
		zip -qr "$dist_root/$archive_base.zip" "$archive_base"
	)
	archive="$dist_root/$archive_base.zip"
else
	(
		cd "$target_build"
		tar -czf "$dist_root/$archive_base.tar.gz" "$archive_base"
	)
	archive="$dist_root/$archive_base.tar.gz"
fi

printf 'Created %s\n' "$archive"
