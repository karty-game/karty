"""Build the checked-in samples using their exact public SDK pins."""
import html
from pathlib import Path
import shutil
import subprocess
import tomllib
from urllib.request import urlopen

ROOT = Path(__file__).resolve().parents[1]
SITE = ROOT / "dist/samples-site"
SAMPLES = ("pong", "ui-demo", "media-lab", "world-camera")


def main():
    if SITE.exists():
        shutil.rmtree(SITE)
    SITE.mkdir(parents=True)
    cli = str(ROOT / "dist/karty")
    installed = set()
    for name in SAMPLES:
        sample = ROOT / "samples" / name
        config = tomllib.loads((sample / "karty.toml").read_text())
        version = config["sdk"]["version"]
        if version not in installed:
            subprocess.run([cli, "sdk", "install", version], check=True)
            installed.add(version)
        subprocess.run([cli, "build", "--target", "web"], cwd=sample, check=True)
        shutil.copytree(sample / "dist/web", SITE / name)
        for notice in ("RUNTIME_LICENSE.md", "SDK_LICENSE.md"):
            url = f"https://github.com/karty-game/karty-sdk/releases/download/sdk-v{version}/{notice}"
            with urlopen(url, timeout=60) as response:
                (SITE / name / notice).write_bytes(response.read())
    links = "".join(f'<li><a href="{name}/">{html.escape(name)}</a></li>' for name in SAMPLES)
    (SITE / "index.html").write_text(
        '<!doctype html><html lang="en"><meta charset="utf-8">'
        '<meta name="viewport" content="width=device-width,initial-scale=1">'
        '<title>Karty samples</title><style>body{font:18px system-ui;max-width:48rem;'
        'margin:4rem auto;padding:0 1rem;background:#101827;color:#eef2ff}'
        'a{color:#91caff}li{margin:1rem 0}</style>'
        '<h1>Karty samples</h1><p>Games and interfaces built with Karty.</p>'
        f'<ul>{links}</ul></html>')


if __name__ == "__main__":
    main()
