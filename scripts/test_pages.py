"""Check preview isolation and validation without network access."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("pages", Path(__file__).with_name("assemble-pages.py"))
pages = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pages)


class PagesTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.state = self.root / "state"
        self.source = self.root / "input"
        self.state.mkdir()
        self.source.mkdir()
        for name in ("index.html", "pong/index.html", "ui-demo/index.html", "media-lab/index.html"):
            path = self.source / name
            path.parent.mkdir(exist_ok=True)
            path.write_text("sample")

    def test_updates_preserve_other_previews_and_cleanup_closed(self):
        pages.assemble(self.state, self.source, "main")
        pages.assemble(self.state, self.source, "pr/1")
        pages.assemble(self.state, self.source, "pr/2")
        (self.source / "pong/index.html").write_text("updated")
        pages.assemble(self.state, self.source, "pr/1")
        self.assertEqual((self.state / "main/pong/index.html").read_text(), "sample")
        self.assertEqual((self.state / "pr/2/pong/index.html").read_text(), "sample")
        self.assertEqual((self.state / "pr/1/pong/index.html").read_text(), "updated")
        pages.assemble(self.state, self.source, "pr/1", remove=True, open_prs=[])
        self.assertFalse((self.state / "pr/1").exists())
        self.assertFalse((self.state / "pr/2").exists())
        self.assertTrue((self.state / "main/index.html").exists())

    def test_rejects_links_and_preserves_previous_preview(self):
        pages.assemble(self.state, self.source, "pr/1")
        (self.source / "pong/leak").symlink_to(self.state / "index.html")
        with self.assertRaises(ValueError):
            pages.assemble(self.state, self.source, "pr/1")
        self.assertTrue((self.state / "pr/1/index.html").exists())

    def test_rejects_path_escape_and_git_metadata(self):
        for path in ("../escape", "pr/../../main", "/tmp/escape", "pr/0"):
            with self.assertRaises(ValueError):
                pages.assemble(self.state, self.source, path)
        (self.source / "pong/.git").mkdir()
        with self.assertRaises(ValueError):
            pages.validate(self.source)

    def test_rejects_incomplete_and_unexpected_artifacts(self):
        (self.source / "CNAME").write_text("unexpected.example")
        with self.assertRaises(ValueError):
            pages.validate(self.source)
        (self.source / "CNAME").unlink()
        (self.source / "pong/index.html").unlink()
        with self.assertRaises(ValueError):
            pages.validate(self.source)


if __name__ == "__main__":
    unittest.main()
