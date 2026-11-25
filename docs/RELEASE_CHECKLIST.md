# Release checklist

Releases are manual. Pushing a version tag builds, signs and tests every
artifact and creates a **draft** GitHub release; a maintainer reviews and
publishes it. A release candidate always comes first; 1.0.0 (and every later
final release) is cut only after its candidate passes everything below.

## Prepare

- [ ] `main` is green in CI on Windows, macOS and Linux.
- [ ] Put the release notes under `## [Unreleased]` before either release type.
- [ ] For a fix release, use `make release`; it increments the semantic patch
      and `BUILD_NUMBER`, promotes the release notes, synchronizes package
      metadata and commits the change.
- [ ] For a feature release, use
      `make release-feature`; it increments the minor version, resets
      `BUILD_NUMBER` to 1 and promotes those notes to the new version section.
- [ ] `CHANGELOG.md` has a `## [<version>]` section describing the release.
- [ ] `THIRD_PARTY_NOTICES.md` matches the bundled components and versions.
- [ ] No high-severity advisories: the *Known vulnerabilities* CI job passes.
- [ ] Release secrets are present and not expired:
      `WINDOWS_CERTIFICATE`, `WINDOWS_CERTIFICATE_PASSWORD`,
      `MACOS_CERTIFICATE`, `MACOS_CERTIFICATE_PASSWORD`,
      `MACOS_SIGNING_IDENTITY`, `MACOS_NOTARY_KEY`, `MACOS_NOTARY_KEY_ID`,
      `MACOS_NOTARY_ISSUER_ID`.

## Tag

```
git tag -s v<version> -m "exBASIC Studio <version>"
git push origin v<version>
```

The Release workflow fails early if the tag does not equal `VERSION` or the
changelog has no entry.

## Automated gates (Release workflow)

Each must pass; none may be skipped.

- [ ] Toolchain built from pinned sources on every platform.
- [ ] Go and frontend tests against the staged toolchain, with
      `EXBASIC_STUDIO_REQUIRE_TOOLS=1`.
- [ ] Windows: installer and installed executable have valid Authenticode
      signatures; silent install, `--self-test`, launch, silent uninstall;
      portable ZIP `--self-test`.
- [ ] macOS: universal binary contains x86_64 and arm64; Developer ID
      signature verifies; notarization ticket stapled; Gatekeeper accepts
      the DMG and the installed application; `--self-test` natively and
      under Rosetta; launch.
- [ ] Linux x64 and arm64: DEB installs with its declared dependencies;
      desktop entry validates; no unresolved shared libraries;
      `--self-test`; launch under a virtual display; clean removal; RPM
      contents and dependencies; AppImage `--self-test` and launch.
- [ ] Nine release files, `SHA256SUMS.txt`, the SPDX bill of materials and
      provenance attestations are attached to the draft.

`--self-test` covers: bundled tool discovery, license notices, compiling a
sample with debug metadata, ATR packaging and reading, booting under the
debugger and stopping on a BASIC line, symbol inspection, helper shutdown,
a terminal session and Git.

## Manual checks on the candidate

Do these on a clean machine or virtual machine for each platform, with the
downloaded artifacts.

- [ ] Checksums match; `gh attestation verify` succeeds.
- [ ] Install without security warnings (SmartScreen / Gatekeeper).
- [ ] Create a project, edit, build, run; the emulator shows the program.
- [ ] Type on the emulator screen; the `H1:` device lists project files; an
      EOS profile boots and files written in EOS appear in `H`.
- [ ] Set a breakpoint, start debugging, step by line and by instruction,
      add a watch, view memory, stop.
- [ ] Introduce a syntax error: it is marked in the editor and listed in
      Problems.
- [ ] Open, import into, extract from and delete in an SDFS and an EOSFS
      ATR image.
- [ ] Terminal: run a command, resize, Ctrl+C, close.
- [ ] Git: init, commit, branch, add a remote, push and pull against a real
      remote with your normal credentials; resolve a merge conflict.
- [ ] Open a folder you did not create: restricted mode; trusting enables
      building.
- [ ] Kill the application with unsaved changes; restart; recovery is offered.
- [ ] Keyboard-only pass and high-contrast theme.
- [ ] Uninstall; the application files are gone.

## Publish

- [ ] Review the draft release notes and files.
- [ ] Publish the draft (candidates are marked as pre-release).
- [ ] For a final release: at least one candidate passed every check above
      with no changes since, other than the version number.

## If a gate fails

Fix on `main`, raise the candidate number and tag again. Never move or
reuse a tag, and never edit the files of a published release.
