"""Merge static output only. This trusted script must never execute PR artifacts."""
from pathlib import Path
import json
import re
import shutil
import stat
import sys


def validate(source):
    total = 0
    files = list(source.rglob("*"))
    if len(files) > 10000:
        raise ValueError("Too many sample files")
    for path in files:
        relative = path.relative_to(source)
        mode = path.lstat().st_mode
        if not (stat.S_ISREG(mode) or stat.S_ISDIR(mode)):
            raise ValueError(f"Non-regular sample entry: {relative}")
        if any(not re.fullmatch(r"[A-Za-z0-9_][A-Za-z0-9_.-]*", part) for part in relative.parts):
            raise ValueError(f"Invalid sample path: {relative}")
        if relative.parts[0] not in ("pong", "ui-demo", "media-lab", "index.html"):
            raise ValueError(f"Unexpected sample entry: {relative}")
        if path.is_file():
            total += path.stat().st_size
    if total > 200 * 1024 * 1024:
        raise ValueError("Sample output exceeds 200 MiB")
    for name in ("index.html", "pong/index.html", "ui-demo/index.html", "media-lab/index.html"):
        if not (source / name).is_file():
            raise ValueError(f"Missing {name}")


def assemble(state, source, destination, remove=False, open_prs=None):
    if not re.fullmatch(r"main|pr/[1-9][0-9]*", destination):
        raise ValueError("Invalid destination")
    if remove and destination == "main":
        raise ValueError("Cannot remove main samples")
    target = state / destination
    if not remove:
        validate(source)  # Validate everything before replacing a working preview.
    if target.exists():
        shutil.rmtree(target)
    if not remove:
        shutil.copytree(source, target)
    if open_prs is not None:
        for preview in (state / "pr").glob("[0-9]*"):
            if preview.is_dir() and preview.name.isdigit() and int(preview.name) not in open_prs:
                shutil.rmtree(preview)
    links = '<li><a href="main/">Latest samples</a></li>' if (state / "main/index.html").exists() else ''
    for preview in sorted((state / "pr").glob("[0-9]*")):
        if preview.is_dir() and preview.name.isdigit():
            links += f'<li><a href="pr/{preview.name}/">PR #{preview.name}</a></li>'
    (state / "index.html").write_text(
        '<!doctype html><html lang="en"><meta charset="utf-8">'
        '<meta name="viewport" content="width=device-width,initial-scale=1">'
        '<title>Karty samples</title><style>body{font:18px system-ui;max-width:48rem;'
        'margin:4rem auto;padding:0 1rem;background:#101827;color:#eef2ff}'
        'a{color:#91caff}li{margin:1rem 0}</style><h1>Karty samples</h1>'
        f'<p>Latest demos and pull request previews.</p><ul>{links}</ul></html>')


if __name__ == "__main__":
    assemble(Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3], sys.argv[4] == "true", json.loads(sys.argv[5]))
