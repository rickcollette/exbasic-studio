#!/usr/bin/env python3
"""Remove only generated EXBASIC Studio build outputs."""

from __future__ import annotations

import argparse
import shutil
from pathlib import Path


PROJECT_ROOT = Path(__file__).resolve().parent.parent
GENERATED_PATHS = (
    "bin",
    "DIST",
    "build/stage",
    "build/toolchain",
    "frontend/dist",
    "frontend/playwright-report",
    "frontend/test-results",
    "third_party/exbasic/source/build",
    "third_party/exoperatingsystem/build",
)


def clean(root: Path = PROJECT_ROOT, *, dry_run: bool = False) -> list[Path]:
    """Remove known generated paths and return the paths that existed."""
    root = root.resolve()
    removed: list[Path] = []

    for relative_path in GENERATED_PATHS:
        candidate = root / relative_path
        resolved = candidate.resolve()

        try:
            resolved.relative_to(root)
        except ValueError as exc:
            raise RuntimeError(
                f"Refusing to clean path outside the project: {candidate}"
            ) from exc

        if not candidate.exists() and not candidate.is_symlink():
            continue

        removed.append(candidate)
        print(f"{'Would remove' if dry_run else 'Removing'} {candidate.relative_to(root)}")
        if dry_run:
            continue

        if candidate.is_symlink() or candidate.is_file():
            candidate.unlink()
        else:
            shutil.rmtree(candidate)

    if not removed:
        print("Nothing to clean.")

    return removed


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Remove generated build and distribution output."
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="show what would be removed without deleting anything",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    clean(dry_run=args.dry_run)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
