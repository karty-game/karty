import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import build
import smoke


class BuildTests(unittest.TestCase):
    def test_exact_pins(self):
        self.assertEqual(build.PINS["tools"]["crunch"]["commit"], "a836f6dbaf2af6bd10e1f1bff0c74ad82ceaa913")
        self.assertEqual(build.PINS["tools"]["materialize"]["commit"], "1a3fe7d052a464d3217ef66d2bce9889d3946792")
        self.assertEqual(build.PINS["tools"]["materialize"]["binary"], "materialize-cli")
        self.assertEqual(set(build.PINS["targets"]), {"linux-amd64", "linux-arm64", "macos-arm64", "windows-amd64"})

    def test_native_targets(self):
        for system, arch, target in [("Linux", "x86_64", "linux-amd64"),
                                     ("Linux", "aarch64", "linux-arm64"),
                                     ("Darwin", "arm64", "macos-arm64"),
                                     ("Windows", "AMD64", "windows-amd64")]:
            with patch("build.platform.system", return_value=system), patch("build.platform.machine", return_value=arch):
                self.assertEqual(build.native_target(), target)

    def test_reject_cross_build_before_writes(self):
        with patch("build.native_target", return_value="linux-amd64"):
            with self.assertRaisesRegex(ValueError, "native build required"):
                build.build(Path("unused"), Path("unused"), "windows-amd64", None, True)

    def test_full_build_requires_shared_lock(self):
        with patch("build.native_target", return_value="linux-amd64"):
            with self.assertRaisesRegex(ValueError, "require --lock-dir"):
                build.build(Path("unused"), Path("unused"), "linux-amd64", None, False)

    def make_archive(self, path, name, link=False):
        with tarfile.open(path, "w:gz") as archive:
            member = tarfile.TarInfo(name)
            if link:
                member.type = tarfile.SYMTYPE
                member.linkname = "../../outside"
                archive.addfile(member)
            else:
                member.size = 4
                archive.addfile(member, io.BytesIO(b"test"))

    def test_safe_subtree_extraction(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / "source.tar.gz"
            self.make_archive(archive, "repo-pin/Materialize/LICENSE")
            build.extract(archive, root / "out", "repo-pin/Materialize/")
            self.assertEqual((root / "out/LICENSE").read_bytes(), b"test")

    def test_reject_archive_escape_and_links(self):
        for name, link in [("repo-pin/../outside", False), ("/absolute", False),
                           ("repo-pin/link", True), ("repo-pin/a\\b", False)]:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                archive = root / "source.tar.gz"
                self.make_archive(archive, name, link)
                with self.assertRaises(ValueError):
                    build.extract(archive, root / "out", "repo-pin/")

    def test_cached_archive_requires_checksum(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "downloads").mkdir()
            (root / "downloads/crunch.tar.gz").write_bytes(b"tampered")
            with self.assertRaisesRegex(ValueError, "checksum mismatch"):
                build.fetch("crunch", root)
            self.assertFalse((root / "sources").exists())

    def test_lock_identity_tracks_manifest_and_lock(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "Cargo.toml").write_text("manifest")
            lock = root / "Cargo.lock"
            lock.write_text("lock")
            identity = build.lock_identity(root, lock)
            lock.write_text("changed")
            self.assertNotEqual(identity, build.lock_identity(root, lock))
            lock.write_text("lock")
            (root / "Cargo.toml").write_text("changed")
            self.assertNotEqual(identity, build.lock_identity(root, lock))

    def test_zip_hashes_permissions_and_stable_container(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bundle = root / "bundle"
            (bundle / "bin").mkdir(parents=True)
            (bundle / "bin/crunch").write_bytes(b"fixture-not-a-real-binary")
            build.write_json(bundle / "metadata.json", {"fixture": True})
            first = root / "first.zip"
            second = root / "second.zip"
            build.bundle_zip(bundle, first)
            # SHA256SUMS is an output, not its own input on a rebundle.
            (bundle / "SHA256SUMS").unlink()
            build.bundle_zip(bundle, second)
            self.assertEqual(first.read_bytes(), second.read_bytes())
            with zipfile.ZipFile(first) as archive:
                checksums = archive.read("SHA256SUMS").decode().splitlines()
                for line in checksums:
                    digest, name = line.split("  ", 1)
                    import hashlib
                    self.assertEqual(digest, hashlib.sha256(archive.read(name)).hexdigest())
                self.assertEqual((archive.getinfo("bin/crunch").external_attr >> 16) & 0o777, 0o755)
                self.assertEqual(archive.getinfo("metadata.json").date_time, (1980, 1, 1, 0, 0, 0))
            self.assertEqual(first.with_suffix(".zip.sha256").read_text().split()[0], build.sha256(first))

    def test_handwritten_patches_have_final_newlines(self):
        for path in build.PATCHES:
            self.assertTrue(path.read_bytes().endswith(b"\n"), path.name)

    def test_smoke_fixture_is_real_png(self):
        image = smoke.png((255, 0, 0, 255))
        self.assertTrue(image.startswith(b"\x89PNG\r\n\x1a\n"))
        self.assertIn(b"IHDR", image)
        self.assertIn(b"IDAT", image)


if __name__ == "__main__":
    unittest.main()