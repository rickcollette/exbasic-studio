"""Unit tests for the local remote-build and publishing helpers."""

from __future__ import annotations

import io
import tempfile
from pathlib import Path
import tarfile
import unittest

import publish_release
import remote_build
import stage_windows_release
import bump_version


class RemoteBuildTests(unittest.TestCase):
    def test_secret_labels_are_parsed_without_values_in_result_keys(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / ".secrets"
            path.write_text("# builders\nMAC: ssh mac-alias\nLINUX: ssh -p 2222 linux-alias\n", encoding="utf-8")
            connections = remote_build.read_connections(path)
        self.assertEqual(set(connections), {"MAC", "LINUX"})
        self.assertTrue(connections["MAC"].startswith("ssh "))

    def test_private_and_generated_paths_are_excluded(self) -> None:
        self.assertTrue(remote_build.is_excluded(".private/.privkey"))
        self.assertTrue(remote_build.is_excluded("build/stage/linux-amd64/tools"))
        self.assertTrue(remote_build.is_excluded("frontend/node_modules/vite/index.js"))
        self.assertTrue(remote_build.is_excluded("third_party/exoperatingsystem/build/EOS.car"))
        self.assertFalse(remote_build.is_excluded("build/remote/run-linux.sh"))
        self.assertFalse(remote_build.is_excluded("third_party/exoperatingsystem/eos/kernel.asm"))

    def test_ssh_identity_and_batch_options_are_forced(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            key = Path(temporary) / "key"
            key.write_text("test", encoding="ascii")
            argv = remote_build.ssh_argv("ssh -i old-key -p 2222 builder", key)
        self.assertEqual(argv[0], "ssh")
        self.assertIn(str(key.resolve()), argv)
        self.assertIn("BatchMode=yes", argv)
        self.assertNotIn("old-key", argv)
        self.assertEqual(argv[-1], "builder")

    def test_artifact_bundle_rejects_parent_paths(self) -> None:
        member = tarfile.TarInfo("../outside")
        with self.assertRaises(remote_build.BuildError):
            remote_build.validate_bundle_member(member)

    def test_artifact_bundle_ignores_macos_metadata(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bundle = root / "bundle.tar.gz"
            destination = root / "out"
            destination.mkdir()
            with tarfile.open(bundle, "w:gz") as archive:
                for name in ("artifact.dmg", "._artifact.dmg", "__MACOSX/._artifact.dmg"):
                    payload = b"release" if name == "artifact.dmg" else b"metadata"
                    member = tarfile.TarInfo(name)
                    member.size = len(payload)
                    archive.addfile(member, io.BytesIO(payload))
            extracted = remote_build.extract_bundle(bundle, destination)
            self.assertEqual(extracted, [destination / "artifact.dmg"])
            self.assertFalse((destination / "._artifact.dmg").exists())


class PublishReleaseTests(unittest.TestCase):
    def test_release_tag_includes_build_number(self) -> None:
        self.assertEqual(publish_release.release_tag("1.0.0", 7), "v1.0.0-build.7")

    def test_github_repository_is_inferred_from_https_and_ssh(self) -> None:
        self.assertEqual(publish_release.infer_repo("https://github.com/acme/widget.git"), "acme/widget")
        self.assertEqual(publish_release.infer_repo("git@github.com:acme/widget.git"), "acme/widget")

    def test_non_github_remote_is_not_inferred(self) -> None:
        self.assertIsNone(publish_release.infer_repo("ssh://git@example.test/acme/widget.git"))


class StageWindowsReleaseTests(unittest.TestCase):
    def test_windows_packages_are_versioned_and_staged(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "bin").mkdir()
            (root / "VERSION").write_text("1.2.3\n", encoding="utf-8")
            installer = root / "bin" / "exbasic-studio-amd64-installer.exe"
            portable = root / "bin" / "exbasic-studio-1.2.3-windows-amd64-portable.zip"
            installer.write_bytes(b"installer")
            portable.write_bytes(b"portable")

            staged = stage_windows_release.stage_windows(root)

            self.assertEqual(
                [path.name for path in staged],
                [
                    "exbasic-studio-1.2.3-windows-amd64-installer.exe",
                    "exbasic-studio-1.2.3-windows-amd64-portable.zip",
                ],
            )
            self.assertEqual(staged[0].read_bytes(), b"installer")
            self.assertEqual(staged[1].read_bytes(), b"portable")


class BumpVersionTests(unittest.TestCase):
    def test_fix_increments_only_the_build(self) -> None:
        self.assertEqual(bump_version.next_identity("1.0.0", 7, "fix"), ("1.0.0", 8))

    def test_feature_increments_minor_and_resets_build(self) -> None:
        self.assertEqual(bump_version.next_identity("1.0.0", 7, "feature"), ("1.1.0", 1))

    def test_feature_promotes_unreleased_changelog_notes(self) -> None:
        source = "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- Preview.\n\n## [1.0.0]\n\n- Initial.\n"
        updated = bump_version.promote_unreleased(source, "1.1.0")
        self.assertIn("## [Unreleased]\n\n## [1.1.0]\n\n### Added", updated)

    def test_feature_requires_unreleased_notes(self) -> None:
        with self.assertRaises(bump_version.BumpError):
            bump_version.promote_unreleased("# Changelog\n\n## [Unreleased]\n\n## [1.0.0]\n", "1.1.0")


if __name__ == "__main__":
    unittest.main()
