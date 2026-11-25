# Remote release builds

The release helper builds Linux artifacts on a Linux SSH host and macOS
artifacts on an Apple Silicon Mac. It uploads a source snapshot rather than
building an arbitrary remote checkout, so the result includes the local source
edits present when the command starts. `.private`, `DIST`, caches and previous
build outputs are never uploaded.

## Private configuration

Create these ignored files:

```text
.private/.privkey
.private/.secrets
```

`.privkey` is an OpenSSH private key. `.secrets` contains one SSH invocation per
host:

```text
LINUX: ssh build-user@linux-builder
MAC: ssh build-user@mac-builder
```

Normal SSH options and aliases from `~/.ssh/config` are supported. The helper
always adds `.private/.privkey`, `BatchMode=yes` and `IdentitiesOnly=yes`. It
never prints the configured commands or host names.

## Builder prerequisites

The Linux host needs Docker and enough free space to build two local images.
The containers pin Go, Node, Wails and Task and provide the remaining build
dependencies. They produce two x64 lanes:

- `compat`: Ubuntu 22.04/glibc 2.35, GTK3 and WebKitGTK 4.1. This is the lane
  for Ubuntu 22.04, 24.04 and 26.04, Debian 12/current and current Fedora.
- `current`: Ubuntu 24.04/glibc 2.39, GTK4 and WebKitGTK 6.0. This is the lane
  for Ubuntu 24.04 and 26.04, Debian current and current Fedora.

Ubuntu 20.04 is intentionally unsupported. The compatibility manifest records
the exact highest referenced glibc symbol so the baseline remains auditable.

The Mac must be Apple Silicon with Rosetta 2, Xcode command-line tools, Go,
Node and network access. The runner installs pinned Wails v3.0.0 and Task
v3.40.1 into a persistent per-user release-tool cache. It first builds them
offline from Go's module cache. If the source has never been cached, it fetches
the pinned tag directly from the official GitHub repository with bounded
retries, avoiding a dependency on `proxy.golang.org`. Go may fetch the required
Go 1.26 toolchain automatically. The script builds and self-tests separate
Intel (`amd64`) and Apple Silicon (`arm64`) application bundles in parallel.
It produces a ZIP and DMG for each architecture. Bundles are ad-hoc signed,
not Developer ID signed or notarized; public distribution without Gatekeeper
warnings still requires Apple credentials.

## Build

Validate without connecting:

```shell
python scripts/remote_build.py --check
```

Build both platforms, or only one:

```shell
python scripts/remote_build.py
python scripts/remote_build.py --target linux
python scripts/remote_build.py --target mac
```

If one remote platform fails after the other completes, retry only the failed
platform. The existing successful artifacts are preserved and the checksum
file is regenerated across all platforms when the retry succeeds:

```shell
make release REMOTE_TARGET=mac
```

Every distributable, a per-lane build manifest and `SHA256SUMS.txt` are written
to `DIST/`. Existing Windows artifacts for the same version are preserved and
included in the checksum file.

From the Windows release workstation, `make dist` builds and stages the
Windows installer and portable ZIP before running both remote builders.

## Push source and create a GitHub release

The publisher is read-only unless `--execute` is present. Because the release
tag must identify the exact source used to build the artifacts, it refuses a
dirty worktree unless `--commit-all` is explicitly selected.

There is no `origin` remote in a new checkout, so provide the repository on the
first run:

```shell
python scripts/publish_release.py --repo OWNER/REPO
python scripts/publish_release.py --repo OWNER/REPO --commit-all --execute
```

The first command is a preflight. The second adds `origin` when needed, commits
all current changes, pushes the branch and annotated version tag, and creates
or updates a draft release. Existing same-named release assets are replaced.

Publishing instead of leaving a draft requires a second explicit switch:

```shell
python scripts/publish_release.py --repo OWNER/REPO --execute --publish-release
```

The GitHub CLI must already be authenticated (`gh auth login`). Never use
`--commit-all` without reviewing `git status`; it deliberately includes every
tracked and untracked source change.

The Makefile wraps the complete versioning, build and publishing flow:

```shell
# Fix release: 1.0.0 build 1 becomes 1.0.1 build 2.
make release

# Feature release: 1.0.0 becomes 1.1.0 and the build resets to 1.
make release-feature

# Publish the current draft without changing its version again.
make release-publish
```

`make release` and `make release-feature` include the current worktree in the
release commit, synchronize the version metadata, and commit everything before
building. Set `COMMIT_ALL=0` to require a clean worktree instead. A failed build
can be retried safely: an untagged release identity is reused rather than
incremented again.
