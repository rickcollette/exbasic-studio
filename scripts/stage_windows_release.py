#!/usr/bin/env python3
"""Copy versioned Windows release packages from bin into DIST."""

from __future__ import annotations

import argparse
from pathlib import Path
import re
import shutil
import sys


ROOT = Path(__file__).resolve().parent.parent


class StageError(RuntimeError):
    pass


def read_version(root: Path = ROOT) -> str:
    value = (root / "VERSION").read_text(encoding="utf-8").strip()
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?", value):
        raise StageError(f"invalid VERSION value: {value!r}")
    return value


def stage_windows(root: Path = ROOT, *, arch: str = "amd64") -> list[Path]:
    if not re.fullmatch(r"[A-Za-z0-9_-]+", arch):
        raise StageError(f"invalid architecture: {arch!r}")

    version = read_version(root)
    bin_dir = root / "bin"
    dist_dir = root / "DIST"
    packages = (
        (
            bin_dir / f"exbasic-studio-{arch}-installer.exe",
            dist_dir / f"exbasic-studio-{version}-windows-{arch}-installer.exe",
        ),
        (
            bin_dir / f"exbasic-studio-{version}-windows-{arch}-portable.zip",
            dist_dir / f"exbasic-studio-{version}-windows-{arch}-portable.zip",
        ),
    )

    missing = [source for source, _ in packages if not source.is_file()]
    if missing:
        names = ", ".join(str(path.relative_to(root)) for path in missing)
        raise StageError(f"missing Windows release package(s): {names}")

    dist_dir.mkdir(parents=True, exist_ok=True)
    staged: list[Path] = []
    for source, destination in packages:
        shutil.copy2(source, destination)
        staged.append(destination)
        print(f"Staged {destination.relative_to(root)}")
    return staged


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--arch", default="amd64", help="Windows architecture (default: amd64)")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        stage_windows(arch=args.arch)
        return 0
    except (StageError, OSError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
