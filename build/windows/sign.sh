#!/usr/bin/env bash
# Authenticode-signs Windows executables with the release certificate.
#
#   build/windows/sign.sh <file-or-directory>...
#
# WINDOWS_CERTIFICATE (base64 PFX) and WINDOWS_CERTIFICATE_PASSWORD come from
# release CI secrets. Contributor builds have neither and stay unsigned; in
# release CI a missing certificate is an error.
set -euo pipefail

if [ -z "${WINDOWS_CERTIFICATE:-}" ]; then
	if [ -n "${GITHUB_ACTIONS:-}" ] && [ "${GITHUB_REF_TYPE:-}" = "tag" ]; then
		echo "sign.sh: WINDOWS_CERTIFICATE is not configured for this release build" >&2
		exit 1
	fi
	echo "sign.sh: no certificate configured; leaving files unsigned"
	exit 0
fi

signtool="$(find "/c/Program Files (x86)/Windows Kits/10/bin" -name signtool.exe -path '*x64*' 2>/dev/null | sort | tail -n 1)"
[ -n "$signtool" ] || { echo "sign.sh: signtool.exe not found" >&2; exit 1; }

pfx="$(mktemp --suffix=.pfx)"
trap 'rm -f "$pfx"' EXIT
printf '%s' "$WINDOWS_CERTIFICATE" | base64 -d > "$pfx"

files=()
for target in "$@"; do
	if [ -d "$target" ]; then
		while IFS= read -r file; do files+=("$file"); done < <(find "$target" -type f -name '*.exe')
	else
		files+=("$target")
	fi
done
[ "${#files[@]}" -gt 0 ] || { echo "sign.sh: nothing to sign" >&2; exit 1; }

MSYS2_ARG_CONV_EXCL='*' "$signtool" sign /fd SHA256 /td SHA256 /tr http://timestamp.digicert.com \
	/f "$(cygpath -w "$pfx")" /p "$WINDOWS_CERTIFICATE_PASSWORD" \
	"${files[@]}"
MSYS2_ARG_CONV_EXCL='*' "$signtool" verify /pa "${files[@]}"