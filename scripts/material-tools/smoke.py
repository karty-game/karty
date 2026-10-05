#!/usr/bin/env python3
"""Exercise a real Crunch atlas; Materialize checks here do not initialize a GPU."""

import argparse
import binascii
import hashlib
import json
from pathlib import Path
import shutil
import struct
import subprocess
import zlib


def png(color):
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", binascii.crc32(kind + data) & 0xffffffff)
    pixels = b"".join(b"\0" + bytes(color) * 4 for _ in range(4))
    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 4, 4, 8, 6, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(pixels)) + chunk(b"IEND", b""))


def smoke(binaries, work, materialize=True):
    work.mkdir(parents=True, exist_ok=True)
    suffix = ".exe" if (binaries / "crunch.exe").exists() else ""
    executable = (binaries / ("crunch" + suffix)).resolve()
    snapshots = []
    for index, names in enumerate([("zeta", "alpha", "middle"), ("middle", "alpha", "zeta")]):
        directory = work / f"case{index}"
        if directory.exists():
            shutil.rmtree(directory)
        directory.mkdir()
        colors = {"alpha": (255, 0, 0, 255), "middle": (0, 255, 0, 255), "zeta": (0, 0, 255, 255)}
        for name in names:
            (directory / (name + ".png")).write_bytes(png(colors[name]))
        # The upstream CLI is OUTPUT, INPUT, options (its error text says otherwise).
        # Deliberately omit rotation, trim, unique and premultiply; this is not an adapter.
        inputs = ",".join(name + ".png" for name in names)
        subprocess.run([str(executable), "atlas", inputs, "-j", "-f", "-s64", "-p0"],
                       cwd=directory, check=True, timeout=60)
        data = json.loads((directory / "atlas.json").read_text())
        images = data["textures"][0]["images"]
        if [item["n"] for item in images] != ["alpha", "middle", "zeta"]:
            raise ValueError("equal-area name tie-break failed")
        if any(item["w"] != 4 or item["h"] != 4 or "r" in item for item in images):
            raise ValueError("unexpected atlas geometry/options")
        image = (directory / "atlas0.png").read_bytes()
        if not image.startswith(b"\x89PNG\r\n\x1a\n"):
            raise ValueError("atlas PNG missing")
        snapshots.append(((directory / "atlas.json").read_bytes(), image))
    if snapshots[0] != snapshots[1]:
        raise ValueError("atlas differs with reversed equal-area input order")
    result = {"crunch": "actual atlas; reversed input order gives identical PNG/JSON",
              "atlas_png_sha256": hashlib.sha256(snapshots[0][1]).hexdigest()}
    if materialize:
        cli = binaries / ("materialize-cli" + suffix)
        for arg in ("--help", "--version", "--list-maps"):
            subprocess.run([str(cli.resolve()), arg], check=True, timeout=60)
        result["materialize"] = "help/version/list-maps executed; no GPU compute validation"
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin", required=True, type=Path)
    parser.add_argument("--work", required=True, type=Path)
    parser.add_argument("--only-crunch", action="store_true")
    args = parser.parse_args()
    print(json.dumps(smoke(args.bin, args.work, not args.only_crunch), indent=2))