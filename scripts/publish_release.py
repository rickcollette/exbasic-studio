#!/usr/bin/env python3
"""Push the current source/tag and upload DIST artifacts to GitHub Releases.

The default mode is a complete read-only preflight.  Pass --execute to push.
Releases are drafts unless --publish-release is also supplied.
"""

from __future__ import annotations

import argparse
import hashlib
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
from typing import Iterable


ROOT = Path(__file__).resolve().parent.parent
DIST = ROOT / "DIST"


class PublishError(RuntimeError):
    pass


def run(*args: str, capture: bool = False, check: bool = True) -> subprocess.CompletedProcess[str]:
    completed = subprocess.run(
        list(args),
        cwd=ROOT,
        text=True,
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.PIPE if capture else None,
        check=False,
    )
    if check and completed.returncode:
        detail = (completed.stderr or completed.stdout or "").strip()
        raise PublishError(f"command failed ({' '.join(args[:3])}): {detail or completed.returncode}")
    return completed


def output(*args: str) -> str:
    return run(*args, capture=True).stdout.strip()


def version() -> str:
    value = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?", value):
        raise PublishError(f"VERSION is not semantic-version shaped: {value!r}")
    return value


def build_number() -> int:
    value = (ROOT / "BUILD_NUMBER").read_text(encoding="utf-8").strip()
    if not value.isascii() or not value.isdigit() or int(value) < 1:
        raise PublishError(f"BUILD_NUMBER is not a positive integer: {value!r}")
    return int(value)


def release_tag(current_version: str, current_build: int) -> str:
    return f"v{current_version}-build.{current_build}"


def release_notes(current_version: str) -> str:
    lines = (ROOT / "CHANGELOG.md").read_text(encoding="utf-8").splitlines()
    heading = f"## [{current_version}]"
    try:
        start = next(index for index, line in enumerate(lines) if line.startswith(heading)) + 1
    except StopIteration as exc:
        raise PublishError(f"CHANGELOG.md has no {heading} section") from exc
    end = next((index for index in range(start, len(lines)) if lines[index].startswith("## [")), len(lines))
    notes = "\n".join(lines[start:end]).strip()
    if not notes:
        raise PublishError(f"the {heading} changelog section is empty")
    return notes + "\n"


def release_assets(current_version: str) -> list[Path]:
    prefix = f"exbasic-studio-{current_version}-"
    assets = sorted(path for path in DIST.glob(f"{prefix}*") if path.is_file())
    checksum = DIST / "SHA256SUMS.txt"
    if checksum.is_file():
        assets.append(checksum)
    if not any(path.suffix.lower() in {".exe", ".zip", ".appimage", ".deb", ".rpm", ".dmg"} for path in assets):
        raise PublishError(f"DIST contains no distributables for {current_version}")
    return assets


def parse_checksums() -> dict[str, str]:
    path = DIST / "SHA256SUMS.txt"
    if not path.is_file():
        raise PublishError("DIST/SHA256SUMS.txt is missing; run scripts/remote_build.py first")
    result: dict[str, str] = {}
    for number, line in enumerate(path.read_text(encoding="ascii").splitlines(), 1):
        match = re.fullmatch(r"([0-9a-f]{64})  (.+)", line)
        if not match:
            raise PublishError(f"SHA256SUMS.txt:{number}: malformed checksum line")
        result[match.group(2)] = match.group(1)
    return result


def verify_assets(assets: list[Path]) -> None:
    checksums = parse_checksums()
    files = [asset for asset in assets if asset.name != "SHA256SUMS.txt"]
    expected_names = {path.name for path in files}
    if set(checksums) != expected_names:
        missing = sorted(expected_names - set(checksums))
        extra = sorted(set(checksums) - expected_names)
        raise PublishError(f"checksum file does not match release assets (missing={missing}, extra={extra})")
    for path in files:
        with path.open("rb") as stream:
            actual = hashlib.file_digest(stream, "sha256").hexdigest()
        if actual != checksums[path.name]:
            raise PublishError(f"checksum mismatch: {path.name}")


def remote_url(remote: str) -> str | None:
    completed = run("git", "remote", "get-url", remote, capture=True, check=False)
    return completed.stdout.strip() if completed.returncode == 0 else None


def infer_repo(url: str | None) -> str | None:
    if not url:
        return None
    match = re.search(r"github\.com[/:]([^/]+/[^/]+?)(?:\.git)?$", url)
    return match.group(1) if match else None


def ensure_remote(remote: str, repo: str | None, execute: bool) -> tuple[str, str]:
    url = remote_url(remote)
    inferred = infer_repo(url)
    selected = repo or inferred
    if not selected or not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", selected):
        raise PublishError("cannot determine GitHub repository; pass --repo OWNER/REPO")
    if url is None:
        url = f"https://github.com/{selected}.git"
        if execute:
            run("git", "remote", "add", remote, url)
        else:
            print(f"Would add Git remote {remote} for {selected}")
    elif repo and inferred and inferred.lower() != repo.lower():
        raise PublishError(f"--repo {repo} does not match {remote} ({inferred})")
    return selected, url


def git_preflight(commit_all: bool, execute: bool, identity: str) -> str:
    branch = output("git", "branch", "--show-current")
    if not branch:
        raise PublishError("detached HEAD is not supported for release publishing")
    dirty = output("git", "status", "--porcelain")
    if dirty and not commit_all:
        raise PublishError("the worktree is dirty; commit it first or explicitly pass --commit-all")
    if dirty and commit_all:
        if execute:
            run("git", "add", "-A")
            run("git", "commit", "-m", f"Release {identity}")
        else:
            count = len(dirty.splitlines())
            print(f"Would commit all {count} changed paths as 'Release {identity}'")
    return branch


def ensure_tag(tag: str, execute: bool) -> None:
    head = output("git", "rev-parse", "HEAD")
    existing = run("git", "rev-parse", "-q", "--verify", f"refs/tags/{tag}^{{}}", capture=True, check=False)
    if existing.returncode == 0:
        if existing.stdout.strip() != head:
            raise PublishError(f"local tag {tag} exists but does not point at HEAD")
        return
    if execute:
        run("git", "tag", "-a", tag, "-m", f"exBASIC Studio {tag.removeprefix('v')}")
    else:
        print(f"Would create annotated tag {tag} at HEAD")


def github_preflight(repo: str, execute: bool) -> None:
    if shutil.which("gh") is None:
        raise PublishError("GitHub CLI (gh) was not found on PATH")
    if execute:
        run("gh", "auth", "status")
        run("gh", "repo", "view", repo, "--json", "nameWithOwner")


def publish_release(
    repo: str,
    remote: str,
    branch: str,
    current_version: str,
    current_build: int,
    assets: list[Path],
    notes_file: Path,
    publish: bool,
) -> None:
    tag = release_tag(current_version, current_build)
    run("git", "push", "--set-upstream", remote, branch)
    run("git", "push", remote, tag)

    exists = run("gh", "release", "view", tag, "--repo", repo, capture=True, check=False).returncode == 0
    prerelease = "-" in current_version
    if exists:
        run("gh", "release", "upload", tag, *(str(path) for path in assets), "--clobber", "--repo", repo)
        edit = [
            "gh", "release", "edit", tag,
            "--repo", repo,
            "--title", f"exBASIC Studio {current_version} (build {current_build})",
            "--notes-file", str(notes_file),
            f"--draft={'false' if publish else 'true'}",
            f"--prerelease={'true' if prerelease else 'false'}",
        ]
        run(*edit)
    else:
        create = [
            "gh", "release", "create", tag,
            *(str(path) for path in assets),
            "--repo", repo,
            "--verify-tag",
            "--title", f"exBASIC Studio {current_version} (build {current_build})",
            "--notes-file", str(notes_file),
        ]
        if not publish:
            create.append("--draft")
        if prerelease:
            create.append("--prerelease")
        run(*create)


def parse_args(argv: Iterable[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", help="GitHub OWNER/REPO (inferred from the remote when possible)")
    parser.add_argument("--remote", default="origin", help="Git remote to push (default: origin)")
    parser.add_argument("--commit-all", action="store_true", help="stage and commit every current worktree change")
    parser.add_argument("--execute", action="store_true", help="perform git pushes and GitHub release changes")
    parser.add_argument(
        "--source-only",
        action="store_true",
        help="validate source, Git, and GitHub prerequisites without requiring DIST assets",
    )
    parser.add_argument(
        "--publish-release",
        action="store_true",
        help="publish immediately instead of creating/updating a draft (requires --execute)",
    )
    return parser.parse_args(argv)


def main(argv: Iterable[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        if args.publish_release and not args.execute:
            raise PublishError("--publish-release requires --execute")
        if args.source_only and args.execute:
            raise PublishError("--source-only cannot be combined with --execute")
        current_version = version()
        current_build = build_number()
        run("go", "run", "./build/tools/version", "--check")
        notes = release_notes(current_version)
        tag = release_tag(current_version, current_build)
        branch = git_preflight(args.commit_all, args.execute, tag)
        repo, _ = ensure_remote(args.remote, args.repo, args.execute)
        github_preflight(repo, args.execute)
        ensure_tag(tag, args.execute)

        if args.source_only:
            print(f"Source preflight passed for {tag}: repository {repo}, branch {branch}.")
            return 0

        assets = release_assets(current_version)
        verify_assets(assets)

        print(f"Preflight passed for {tag}: {len(assets)} assets, repository {repo}, branch {branch}.")
        if not args.execute:
            print("Dry run only. Re-run with --execute to push and create/update the draft release.")
            return 0

        with tempfile.TemporaryDirectory(prefix="exbasic-release-notes-") as temporary:
            notes_path = Path(temporary) / "notes.md"
            notes_path.write_text(notes, encoding="utf-8", newline="\n")
            publish_release(
                repo,
                args.remote,
                branch,
                current_version,
                current_build,
                assets,
                notes_path,
                args.publish_release,
            )
        state = "published" if args.publish_release else "draft"
        print(f"GitHub release {tag} is {state} and its assets are synchronized.")
        return 0
    except (PublishError, OSError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
