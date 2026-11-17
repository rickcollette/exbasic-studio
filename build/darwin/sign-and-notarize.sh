#!/usr/bin/env bash
# Signs a macOS application bundle with a Developer ID certificate (hardened
# runtime, secure timestamp), builds the DMG, notarizes it and staples the
# ticket.
#
#   build/darwin/sign-and-notarize.sh <App.app> <output.dmg>
#
# Credentials come from release CI secrets:
#   MACOS_CERTIFICATE (base64 .p12), MACOS_CERTIFICATE_PASSWORD,
#   MACOS_SIGNING_IDENTITY ("Developer ID Application: Name (TEAMID)"),
#   NOTARY_KEY (base64 App Store Connect API key .p8), NOTARY_KEY_ID,
#   NOTARY_ISSUER_ID.
set -euo pipefail

app="$1"; dmg="$2"
for name in MACOS_CERTIFICATE MACOS_CERTIFICATE_PASSWORD MACOS_SIGNING_IDENTITY NOTARY_KEY NOTARY_KEY_ID NOTARY_ISSUER_ID; do
	[ -n "${!name:-}" ] || { echo "sign-and-notarize: $name is not configured" >&2; exit 1; }
done

work="$(mktemp -d)"
keychain="$work/release.keychain-db"
password="$(uuidgen)"
cleanup() { security delete-keychain "$keychain" >/dev/null 2>&1 || true; rm -rf "$work"; }
trap cleanup EXIT

printf '%s' "$MACOS_CERTIFICATE" | base64 --decode > "$work/certificate.p12"
printf '%s' "$NOTARY_KEY" | base64 --decode > "$work/notary.p8"
security create-keychain -p "$password" "$keychain"
security set-keychain-settings -lut 3600 "$keychain"
security unlock-keychain -p "$password" "$keychain"
security import "$work/certificate.p12" -k "$keychain" -P "$MACOS_CERTIFICATE_PASSWORD" -T /usr/bin/codesign
security set-key-partition-list -S apple-tool:,apple: -s -k "$password" "$keychain" >/dev/null
security list-keychains -d user -s "$keychain" $(security list-keychains -d user | tr -d '"')

sign() { codesign --force --options runtime --timestamp --keychain "$keychain" --sign "$MACOS_SIGNING_IDENTITY" "$@"; }

# Sign from the inside out: every bundled tool first, then the application.
while IFS= read -r -d '' file; do
	if file -b "$file" | grep -q 'Mach-O'; then sign "$file"; fi
done < <(find "$app/Contents/Resources" -type f -print0)
sign "$app/Contents/MacOS/"*
sign "$app"
codesign --verify --deep --strict --verbose=2 "$app"

staging="$work/dmg"
mkdir -p "$staging"
cp -R "$app" "$staging/"
ln -s /Applications "$staging/Applications"
hdiutil create -volname "exBASIC Studio" -srcfolder "$staging" -ov -format UDZO "$dmg"
sign "$dmg"

xcrun notarytool submit "$dmg" --key "$work/notary.p8" --key-id "$NOTARY_KEY_ID" --issuer "$NOTARY_ISSUER_ID" --wait
xcrun stapler staple "$dmg"
xcrun stapler validate "$dmg"
echo "Signed and notarized: $dmg"