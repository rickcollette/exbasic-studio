#!/usr/bin/env bash
# Merges an x86_64 and an arm64 application bundle into one universal bundle.
# Every Mach-O file (the application and each bundled tool) is combined with
# lipo; all other files must be identical and are copied from the arm64 one.
#
#   build/darwin/make-universal.sh <amd64.app> <arm64.app> <output.app>
set -euo pipefail

amd64="$1"; arm64="$2"; out="$3"
rm -rf "$out"
mkdir -p "$(dirname "$out")"
cp -R "$arm64" "$out"

while IFS= read -r -d '' file; do
	rel="${file#"$arm64"/}"
	if file -b "$file" | grep -q 'Mach-O'; then
		[ -f "$amd64/$rel" ] || { echo "make-universal: $rel is missing from the x86_64 bundle" >&2; exit 1; }
		lipo -create -output "$out/$rel" "$amd64/$rel" "$file"
		lipo -archs "$out/$rel" | grep -q x86_64
		lipo -archs "$out/$rel" | grep -q arm64
	elif ! cmp -s "$file" "$amd64/$rel"; then
		# Libraries built for the Atari target and data files do not depend on
		# the host architecture; a difference means the builds diverged.
		echo "make-universal: $rel differs between the two builds" >&2
		exit 1
	fi
done < <(find "$arm64" -type f -print0)

# Signatures of the inputs do not survive lipo; the bundle is signed next.
find "$out" -name _CodeSignature -type d -prune -exec rm -rf {} +
echo "Universal bundle: $out"