#!/usr/bin/env python3
"""Advance and synchronize the release identity before building artifacts."""

from __future__ import annotations

import argparse
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parent.parent
VERSION_FILES = (
    "VERSION",
    "BUILD_NUMBER",
    "build/config.yml",
    "build/windows/info.json",
    "build/linux/nfpm/nfpm.yaml",
    "build/darwin/Info.plist",
    "build/darwin/Info.dev.plist",
    "frontend/package.json",
    "frontend/package-lock.json",
)


class BumpError(RuntimeError):
    pass


def run(root: Path, *args: str, capture: bool = False, check: bool = True) -> subprocess.CompletedProcess[str]:
    completed = subprocess.run(
        list(args),
        cwd=root,
        text=True,
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.PIPE if capture else None,
        check=False,
    )
    if check and completed.returncode:
        detail = (completed.stderr or completed.stdout or "").strip()
        raise BumpError(f"command failed ({' '.join(args[:3])}): {detail or completed.returncode}")
    return completed


def read_identity(root: Path = ROOT) -> tuple[str, int]:
    version = (root / "VERSION").read_text(encoding="utf-8").strip()
    build_text = (root / "BUILD_NUMBER").read_text(encoding="utf-8").strip()
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise BumpError(f"VERSION must be a final MAJOR.MINOR.PATCH version: {version!r}")
    if not build_text.isascii() or not build_text.isdigit() or int(build_text) < 1:
        raise BumpError(f"BUILD_NUMBER must be a positive integer: {build_text!r}")
    return version, int(build_text)


def release_tag(version: str, build: int) -> str:
    return f"v{version}-build.{build}"


def next_identity(version: str, build: int, kind: str) -> tuple[str, int]:
    major, minor, _patch = (int(part) for part in version.split("."))
    if kind == "fix":
        return version, build + 1
    if kind == "feature":
        return f"{major}.{minor + 1}.0", 1
    raise BumpError(f"unsupported release kind: {kind!r}")


def tag_exists(root: Path, tag: str, remote: str) -> bool:
    local = run(root, "git", "rev-parse", "-q", "--verify", f"refs/tags/{tag}^{{}}", capture=True, check=False)
    if local.returncode == 0:
        return True
    remote_result = run(
        root,
        "git",
        "ls-remote",
        "--exit-code",
        "--tags",
        remote,
        f"refs/tags/{tag}",
        capture=True,
        check=False,
    )
    return remote_result.returncode == 0


def promote_unreleased(text: str, version: str) -> str:
    lines = text.splitlines()
    try:
        heading = lines.index("## [Unreleased]")
    except ValueError as exc:
        raise BumpError("CHANGELOG.md has no ## [Unreleased] section") from exc
    next_heading = next(
        (index for index in range(heading + 1, len(lines)) if lines[index].startswith("## [")),
        len(lines),
    )
    content = lines[heading + 1 : next_heading]
    if not any(line.strip() for line in content):
        raise BumpError("the Unreleased changelog section is empty; add feature release notes first")
    updated = [
        *lines[: heading + 1],
        "",
        f"## [{version}]",
        *content,
        *lines[next_heading:],
    ]
    return "\n".join(updated).rstrip() + "\n"


def bump(root: Path, *, kind: str, remote: str) -> tuple[str, int, bool]:
    version, build = read_identity(root)
    current_tag = release_tag(version, build)
    changed = tag_exists(root, current_tag, remote)
    if not changed:
        print(f"{current_tag} is not tagged yet; reusing it for this release attempt")
        return version, build, False

    new_version, new_build = next_identity(version, build, kind)
    if kind == "feature":
        changelog = root / "CHANGELOG.md"
        updated = promote_unreleased(changelog.read_text(encoding="utf-8"), new_version)
        changelog.write_text(updated, encoding="utf-8", newline="\n")

    (root / "VERSION").write_text(new_version + "\n", encoding="ascii", newline="\n")
    (root / "BUILD_NUMBER").write_text(str(new_build) + "\n", encoding="ascii", newline="\n")
    print(f"Advanced {current_tag} to {release_tag(new_version, new_build)} ({kind})")
    return new_version, new_build, True


def commit_changes(root: Path, tag: str, *, commit_all: bool) -> None:
    if commit_all:
        run(root, "git", "add", "-A")
    else:
        paths = [*VERSION_FILES]
        if run(root, "git", "status", "--porcelain", "--", "CHANGELOG.md", capture=True).stdout.strip():
            paths.append("CHANGELOG.md")
        run(root, "git", "add", "--", *paths)
    staged = run(root, "git", "diff", "--cached", "--quiet", check=False)
    if staged.returncode == 0:
        print(f"No version commit needed for {tag}")
        return
    if staged.returncode != 1:
        raise BumpError("could not inspect staged version changes")
    run(root, "git", "commit", "-m", f"Release {tag}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kind", choices=("fix", "feature"), default="fix")
    parser.add_argument("--remote", default="origin")
    parser.add_argument("--commit", action="store_true", help="commit the synchronized release identity")
    parser.add_argument("--commit-all", action="store_true", help="include all current worktree changes in the release commit")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        dirty = run(ROOT, "git", "status", "--porcelain", capture=True).stdout.strip()
        if dirty and not args.commit_all:
            raise BumpError("the worktree is dirty; commit it first or explicitly use --commit-all")
        version, build, _changed = bump(ROOT, kind=args.kind, remote=args.remote)
        run(ROOT, "go", "run", "./build/tools/version")
        tag = release_tag(version, build)
        if args.commit:
            commit_changes(ROOT, tag, commit_all=args.commit_all)
        print(f"Release identity is {tag}")
        return 0
    except (BumpError, OSError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
