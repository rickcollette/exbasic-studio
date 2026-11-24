#!/usr/bin/env python3
"""Build Linux and macOS release artifacts on configured SSH hosts.

The SSH connection commands are read from .private/.secrets:

    MAC: ssh build-mac
    LINUX: ssh build-linux

The command may contain normal ssh options.  The identity from
.private/.privkey is always added with IdentitiesOnly=yes.  Secret values and
host names are intentionally never printed.
"""

from __future__ import annotations

import argparse
from contextlib import contextmanager
import hashlib
import os
from pathlib import Path, PurePosixPath
import secrets
import shlex
import shutil
import subprocess
import sys
import tarfile
import tempfile
from typing import BinaryIO, Iterable


ROOT = Path(__file__).resolve().parent.parent
PRIVATE = ROOT / ".private"
SECRETS_FILE = PRIVATE / ".secrets"
KEY_FILE = PRIVATE / ".privkey"
DIST = ROOT / "DIST"

EXCLUDED_PREFIXES = (
    ".git",
    ".private",
    ".task",
    "DIST",
    "bin",
    "build/stage",
    "build/toolchain",
    "frontend/dist",
    "frontend/node_modules",
    "frontend/playwright-report",
    "frontend/test-results",
    "third_party/exbasic/source/build",
    "third_party/exoperatingsystem/build",
)


class BuildError(RuntimeError):
    pass


def read_version() -> str:
    version = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
    if not version or any(c not in "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.-" for c in version):
        raise BuildError(f"invalid VERSION value: {version!r}")
    return version


def read_build_number() -> int:
    value = (ROOT / "BUILD_NUMBER").read_text(encoding="utf-8").strip()
    if not value.isascii() or not value.isdigit() or int(value) < 1:
        raise BuildError(f"invalid BUILD_NUMBER value: {value!r}")
    return int(value)


def read_connections(path: Path = SECRETS_FILE) -> dict[str, str]:
    if not path.is_file():
        raise BuildError(f"missing SSH configuration: {path}")
    result: dict[str, str] = {}
    for number, raw in enumerate(path.read_text(encoding="utf-8-sig").splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if ":" not in line:
            raise BuildError(f"{path}:{number}: expected LABEL: ssh-command")
        label, command = line.split(":", 1)
        label = label.strip().upper()
        command = command.strip()
        if not label or not command:
            raise BuildError(f"{path}:{number}: empty label or command")
        if label in result:
            raise BuildError(f"{path}:{number}: duplicate {label} entry")
        result[label] = command
    return result


def without_identity_options(arguments: list[str]) -> list[str]:
    result: list[str] = []
    index = 0
    while index < len(arguments):
        argument = arguments[index]
        lowered = argument.lower()
        if argument == "-i":
            index += 2
            continue
        if argument.startswith("-i") and len(argument) > 2:
            index += 1
            continue
        if lowered == "-o" and index + 1 < len(arguments) and arguments[index + 1].lower().startswith("identityfile="):
            index += 2
            continue
        if lowered.startswith("-oidentityfile="):
            index += 1
            continue
        result.append(argument)
        index += 1
    return result


@contextmanager
def protected_private_key(source: Path = KEY_FILE) -> Iterable[Path]:
    if not source.is_file():
        raise BuildError(f"missing SSH private key: {source}")
    with tempfile.TemporaryDirectory(prefix="exbasic-ssh-key-") as temporary:
        protected = Path(temporary) / "id_release"
        shutil.copyfile(source, protected)
        if os.name == "nt":
            whoami = subprocess.run(["whoami"], text=True, capture_output=True, check=False)
            if whoami.returncode or not whoami.stdout.strip():
                raise BuildError("could not determine the Windows account for SSH key ACLs")
            acl = subprocess.run(
                ["icacls", str(protected), "/inheritance:r", "/grant:r", f"{whoami.stdout.strip()}:(R)"],
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                check=False,
            )
            if acl.returncode:
                raise BuildError(f"could not protect the temporary SSH key: {acl.stderr.strip()}")
        else:
            protected.chmod(0o600)
        yield protected


def ssh_argv(command: str, key_file: Path = KEY_FILE) -> list[str]:
    if not key_file.is_file():
        raise BuildError(f"missing SSH private key: {key_file}")
    try:
        argv = shlex.split(command, posix=True)
    except ValueError as exc:
        raise BuildError(f"invalid SSH command syntax: {exc}") from exc
    if len(argv) < 2 or Path(argv[0]).name.lower() not in {"ssh", "ssh.exe"}:
        raise BuildError("each connection must be an ssh command ending in one destination")
    forbidden = {";", "&&", "||", "|", ">", "<"}
    if any(arg in forbidden for arg in argv):
        raise BuildError("shell operators are not allowed in SSH connection commands")
    connection_arguments = without_identity_options(argv[1:])
    return [
        argv[0],
        "-i",
        str(key_file.resolve()),
        "-o",
        "BatchMode=yes",
        "-o",
        "IdentitiesOnly=yes",
        "-o",
        "StrictHostKeyChecking=accept-new",
        *connection_arguments,
    ]


def is_excluded(relative: str) -> bool:
    relative = relative.replace("\\", "/").strip("/")
    return any(relative == prefix or relative.startswith(prefix + "/") for prefix in EXCLUDED_PREFIXES)


def create_source_archive(destination: Path) -> None:
    def archive_filter(info: tarfile.TarInfo) -> tarfile.TarInfo | None:
        relative = info.name.removeprefix("source/")
        if relative and is_excluded(relative):
            return None
        return info

    with tarfile.open(destination, "w:gz", compresslevel=6) as archive:
        archive.add(ROOT, arcname="source", recursive=True, filter=archive_filter)


def run_ssh(
    connection: list[str],
    remote_command: str,
    *,
    stdin: BinaryIO | None = None,
    stdout: BinaryIO | int | None = None,
    capture: bool = False,
) -> subprocess.CompletedProcess[bytes]:
    kwargs: dict[str, object] = {"cwd": ROOT, "check": False}
    if stdin is not None:
        kwargs["stdin"] = stdin
    if capture:
        kwargs["stdout"] = subprocess.PIPE
        kwargs["stderr"] = subprocess.PIPE
    elif stdout is not None:
        kwargs["stdout"] = stdout
    completed = subprocess.run([*connection, remote_command], **kwargs)  # type: ignore[arg-type]
    if completed.returncode:
        if capture and completed.stderr:
            sys.stderr.write(completed.stderr.decode("utf-8", errors="replace"))
        raise BuildError(f"remote command failed with exit code {completed.returncode}")
    return completed


def validate_bundle_member(member: tarfile.TarInfo) -> None:
    path = PurePosixPath(member.name)
    if path.is_absolute() or ".." in path.parts:
        raise BuildError(f"unsafe path in artifact bundle: {member.name}")
    if member.issym() or member.islnk() or member.isdev():
        raise BuildError(f"unsupported entry in artifact bundle: {member.name}")


def is_macos_metadata(member: tarfile.TarInfo) -> bool:
    return any(part == "__MACOSX" or part.startswith("._") for part in PurePosixPath(member.name).parts)


def extract_bundle(bundle: Path, destination: Path) -> list[Path]:
    extracted: list[Path] = []
    with tarfile.open(bundle, "r:gz") as archive:
        members = archive.getmembers()
        for member in members:
            validate_bundle_member(member)
        members = [member for member in members if not is_macos_metadata(member)]
        archive.extractall(destination, members=members, filter="data")
        for member in members:
            if member.isfile():
                extracted.append(destination / PurePosixPath(member.name))
    return extracted


def remote_job(
    label: str,
    expected_os: str,
    command: str,
    source_archive: Path,
    version: str,
    keep_remote: bool,
    key_file: Path,
) -> list[Path]:
    connection = ssh_argv(command, key_file)
    token = secrets.token_hex(8)
    job_name = f"exbasic-studio-build-{version}-{token}"
    setup = (
        f'job_root="${{TMPDIR:-/tmp}}/{job_name}"; '
        'mkdir -p "$job_root/source"; tar -xzf - -C "$job_root"'
    )

    print(f"[{label}] checking the configured SSH host")
    probe = run_ssh(connection, "uname -s", capture=True)
    actual_os = (probe.stdout or b"").decode("utf-8", errors="replace").strip()
    if actual_os != expected_os:
        raise BuildError(f"{label} entry reached {actual_os or 'an unknown OS'}, expected {expected_os}")

    remote_root = f'"${{TMPDIR:-/tmp}}/{job_name}"'
    bundle: Path | None = None
    uploaded = False
    try:
        print(f"[{label}] uploading source snapshot")
        with source_archive.open("rb") as stream:
            run_ssh(connection, setup, stdin=stream)
        uploaded = True

        runner = "run-linux.sh" if label == "LINUX" else "run-macos.sh"
        print(f"[{label}] building release artifacts")
        build_command = (
            f"set -e; job_root={remote_root}; "
            'cd "$job_root/source"; '
            f'bash packaging/remote/{runner} "$job_root/artifacts"; '
            'tar -C "$job_root/artifacts" -czf "$job_root/artifacts.tar.gz" .'
        )
        run_ssh(connection, build_command)

        with tempfile.NamedTemporaryFile(prefix=f"{label.lower()}-", suffix=".tar.gz", delete=False) as temp:
            bundle = Path(temp.name)
            print(f"[{label}] downloading artifacts")
            run_ssh(
                connection,
                f"job_root={remote_root}; cat \"$job_root/artifacts.tar.gz\"",
                stdout=temp,
            )

        DIST.mkdir(parents=True, exist_ok=True)
        paths = extract_bundle(bundle, DIST)
    finally:
        if bundle is not None:
            bundle.unlink(missing_ok=True)
        if uploaded and not keep_remote:
            cleanup = (
                f"job_root={remote_root}; "
                'case "$job_root" in "${TMPDIR:-/tmp}"/exbasic-studio-build-*) '
                'rm -rf -- "$job_root" ;; *) exit 2 ;; esac'
            )
            run_ssh(connection, cleanup)

    current = [path for path in paths if path.is_file()]
    if not current:
        raise BuildError(f"{label} returned no artifacts")
    print(f"[{label}] retrieved {len(current)} files")
    return current


def checksum_files(version: str) -> list[Path]:
    prefix = f"exbasic-studio-{version}-"
    return sorted(
        path
        for path in DIST.iterdir()
        if path.is_file() and path.name != "SHA256SUMS.txt" and path.name.startswith(prefix)
    )


def write_checksums(version: str) -> None:
    files = checksum_files(version)
    if not files:
        raise BuildError(f"DIST contains no artifacts for {version}")
    lines: list[str] = []
    for path in files:
        with path.open("rb") as stream:
            digest = hashlib.file_digest(stream, "sha256").hexdigest()
        lines.append(f"{digest}  {path.name}\n")
    (DIST / "SHA256SUMS.txt").write_text("".join(lines), encoding="ascii", newline="\n")


def parse_args(argv: Iterable[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--target",
        choices=("all", "linux", "mac"),
        default="all",
        help="remote platform to build (default: all)",
    )
    parser.add_argument(
        "--keep-remote",
        action="store_true",
        help="leave the temporary remote source and build directories for debugging",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="validate local configuration without connecting or building",
    )
    return parser.parse_args(argv)


def main(argv: Iterable[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        version = read_version()
        build_number = read_build_number()
        connections = read_connections()
        targets = ["LINUX", "MAC"] if args.target == "all" else [args.target.upper()]
        missing = [target for target in targets if target not in connections]
        if missing:
            raise BuildError(f"missing {', '.join(missing)} entry in {SECRETS_FILE}")
        if shutil.which("ssh") is None:
            raise BuildError("ssh was not found on PATH")
        with protected_private_key() as key_file:
            for target in targets:
                ssh_argv(connections[target], key_file)
            print(f"Configuration is valid for {', '.join(targets)} (version {version}, build {build_number}); secret hosts are redacted.")
            if args.check:
                return 0

            with tempfile.TemporaryDirectory(prefix="exbasic-source-") as temporary:
                archive = Path(temporary) / "source.tar.gz"
                print("Creating source snapshot (private files, caches, and previous outputs are excluded)")
                create_source_archive(archive)
                size_mib = archive.stat().st_size / (1024 * 1024)
                print(f"Source snapshot: {size_mib:.1f} MiB")
                for target in targets:
                    remote_job(
                        target,
                        "Linux" if target == "LINUX" else "Darwin",
                        connections[target],
                        archive,
                        version,
                        args.keep_remote,
                        key_file,
                    )
        write_checksums(version)
        print(f"Release artifacts and checksums are in {DIST}")
        return 0
    except (BuildError, OSError, tarfile.TarError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
