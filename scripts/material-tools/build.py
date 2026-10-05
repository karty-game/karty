#!/usr/bin/env python3
"""Native asset-tool builds. Only generated work directories are modified."""

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import platform
import shutil
import stat
import subprocess
import tarfile
import urllib.request
import zipfile

HERE = Path(__file__).resolve().parent
CONFIG = HERE / "sources.json"
PINS = json.loads(CONFIG.read_text())
PATCHES = [HERE / "crunch-order.patch", HERE / "crunch-msvc.patch"]


def sha256(path):
    digest = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def write_json(path, value):
    Path(path).write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def run(args, cwd=None, capture=False):
    args = [str(arg) for arg in args]
    print("+", subprocess.list2cmdline(args), flush=True)
    result = subprocess.run(args, cwd=cwd, check=True, text=True,
                            stdout=subprocess.PIPE if capture else None,
                            stderr=None)
    return result.stdout if capture else None


def extract(archive, destination, prefix):
    """Extract only regular files/directories under the expected source prefix."""
    with tarfile.open(archive, "r:gz") as source:
        for member in source:
            name = PurePosixPath(member.name)
            if name.is_absolute() or ".." in name.parts or "\\" in member.name:
                raise ValueError("unsafe source archive path")
            if member.name == prefix.rstrip("/"):
                continue
            if not member.name.startswith(prefix):
                continue
            relative = member.name[len(prefix):]
            if not relative:
                continue
            target = destination / relative
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
            elif member.isfile():
                target.parent.mkdir(parents=True, exist_ok=True)
                with source.extractfile(member) as src, target.open("wb") as dst:
                    shutil.copyfileobj(src, dst)
            else:
                raise ValueError("source archive contains a link or special file")


def fetch(tool, work):
    pin = PINS["tools"][tool]
    cache = work / "downloads"
    cache.mkdir(parents=True, exist_ok=True)
    archive = cache / (tool + ".tar.gz")
    owner_repo = pin["repository"].removeprefix("https://github.com/")
    if not archive.exists():
        url = f"https://codeload.github.com/{owner_repo}/tar.gz/{pin['commit']}"
        temporary = archive.with_suffix(".partial")
        with urllib.request.urlopen(url, timeout=180) as src, temporary.open("wb") as dst:
            shutil.copyfileobj(src, dst)
        temporary.replace(archive)
    if sha256(archive) != pin["archive_sha256"]:
        raise ValueError(f"{tool} source archive checksum mismatch: {archive}")
    destination = work / "sources" / tool
    if destination.exists():
        shutil.rmtree(destination)
    destination.mkdir(parents=True)
    prefix = f"{owner_repo.split('/')[1]}-{pin['commit']}/"
    if pin["subdirectory"]:
        prefix += pin["subdirectory"] + "/"
    extract(archive, destination, prefix)
    if not (destination / "LICENSE").is_file():
        raise ValueError("source archive missing expected source/license")
    return destination


def cargo(*args, cwd=None, capture=False):
    return run(["cargo", "+" + PINS["rust"], *args], cwd=cwd, capture=capture)


def rust_version():
    version = run(["rustc", "+" + PINS["rust"], "--version", "--verbose"], capture=True)
    if f"release: {PINS['rust']}\n" not in version:
        raise ValueError("unexpected Rust toolchain")
    return version


def lock_identity(source, lock):
    return {"sources_sha256": sha256(CONFIG), "rust": PINS["rust"],
            "manifest_sha256": sha256(source / "Cargo.toml"),
            "lock_sha256": sha256(lock)}


def prepare_lock(work, output):
    rust_version()
    source = fetch("materialize", work)
    if (source / "Cargo.lock").exists():
        raise ValueError("upstream unexpectedly contains Cargo.lock; review lock policy")
    cargo("generate-lockfile", cwd=source)
    output.mkdir(parents=True, exist_ok=True)
    lock = output / "Cargo.lock"
    shutil.copyfile(source / "Cargo.lock", lock)
    write_json(output / "lock.json", lock_identity(source, lock))
    shutil.copyfile(CONFIG, output / "sources.json")


def native_target():
    system = {"Linux": "linux", "Darwin": "macos", "Windows": "windows"}[platform.system()]
    machine = platform.machine().lower()
    arch = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}[machine]
    return system + "-" + arch


def crunch(source, binary, work):
    for patch in PATCHES:
        run(["git", "apply", "--check", patch], cwd=source)
        run(["git", "apply", patch], cwd=source)
    cpp = sorted((source / "crunch").glob("*.cpp"))
    objects = work / "objects"
    objects.mkdir(parents=True, exist_ok=True)
    if platform.system() == "Windows":
        # Upstream v140 project uses Unicode. Modern MSVC, not MinGW.
        compiler = subprocess.run(["cl"], text=True, stdout=subprocess.PIPE,
                                  stderr=subprocess.STDOUT, check=False).stdout
        command = ["cl", "/nologo", "/std:c++14", "/O2", "/EHsc", "/MT",
                   "/DUNICODE", "/D_UNICODE", "/DNOMINMAX", "/D_CRT_SECURE_NO_WARNINGS",
                   "/FIcstring", *cpp, "/Fe:" + str(binary), "/link", "/Brepro"]
    else:
        executable = os.environ.get("CXX", "c++")
        compiler = run([executable, "--version"], capture=True)
        command = [executable, "-std=c++11", "-O2", "-include", "cstring", *cpp, "-o", binary]
    run(command, cwd=objects)
    return {"compiler": compiler, "command": [str(arg) for arg in command]}


def rust_notices(source, bundle):
    cargo("fetch", "--locked", cwd=source)
    metadata = json.loads(cargo("metadata", "--locked", "--format-version", "1", cwd=source, capture=True))
    packages = []
    for package in sorted(metadata["packages"], key=lambda item: item["id"]):
        root = Path(package["manifest_path"]).parent
        files = set()
        for entry in root.iterdir():
            if entry.name.lower().startswith(("license", "licence", "copying", "copyright", "notice")):
                if entry.is_file():
                    files.add(entry)
                elif entry.is_dir():
                    files.update(p for p in entry.rglob("*") if p.is_file())
        if package.get("license_file"):
            files.add(root / package["license_file"])
        copied = []
        for file in sorted(files):
            relative = file.resolve().relative_to(root.resolve())
            target = bundle / "notices" / "rust" / (package["name"] + "-" + package["version"]) / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(file, target)
            copied.append(target.relative_to(bundle).as_posix())
        packages.append({"name": package["name"], "version": package["version"],
                         "source": package["source"], "license": package["license"],
                         "notice_files": copied})
    write_json(bundle / "rust-dependencies.json", packages)
    return [p["name"] + "-" + p["version"] for p in packages if not p["notice_files"]]


def bundle_zip(bundle, output):
    files = sorted(p for p in bundle.rglob("*") if p.is_file())
    (bundle / "SHA256SUMS").write_text("".join(
        f"{sha256(p)}  {p.relative_to(bundle).as_posix()}\n" for p in files), encoding="utf-8")
    with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for path in sorted(p for p in bundle.rglob("*") if p.is_file()):
            relative = path.relative_to(bundle).as_posix()
            info = zipfile.ZipInfo(relative, date_time=(1980, 1, 1, 0, 0, 0))
            info.create_system = 3
            mode = 0o755 if relative.startswith("bin/") else 0o644
            info.external_attr = (stat.S_IFREG | mode) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, path.read_bytes(), compresslevel=9)
    output.with_suffix(".zip.sha256").write_text(f"{sha256(output)}  {output.name}\n", encoding="utf-8")


def build(work, output, target, lock_dir, only_crunch):
    if native_target() != target:
        raise ValueError(f"native build required: running {native_target()}, requested {target}")
    if not only_crunch and not lock_dir:
        raise ValueError("full builds require --lock-dir; dependencies must be resolved once")
    bundle = work / "bundles" / target
    if bundle.exists():
        shutil.rmtree(bundle)
    (bundle / "bin").mkdir(parents=True)
    suffix = ".exe" if target.startswith("windows") else ""
    source = fetch("crunch", work)
    crunch_build = crunch(source, bundle / "bin" / ("crunch" + suffix), work)
    # Retain full embedded third-party notices (LodePNG, tinydir, bin packers).
    shutil.copytree(source, bundle / "notices" / "crunch-source")
    provenance = {"schema": 1, "target": target, "rust_target": PINS["targets"][target],
                  "sources": PINS, "crunch_build": crunch_build,
                  "patches": {p.name: sha256(p) for p in PATCHES},
                  "runner": {"system": platform.platform(), "image_os": os.environ.get("ImageOS"),
                             "image_version": os.environ.get("ImageVersion")},
                  "tools_built": ["crunch"]}
    if not only_crunch:
        provenance["rustc"] = rust_version()
        source = fetch("materialize", work)
        lock = lock_dir / "Cargo.lock"
        if lock_identity(source, lock) != json.loads((lock_dir / "lock.json").read_text()):
            raise ValueError("shared lockfile provenance mismatch")
        shutil.copyfile(lock, source / "Cargo.lock")
        cargo("build", "--release", "--locked", "--bin", "materialize-cli",
              "--target", PINS["targets"][target], cwd=source)
        binary = source / "target" / PINS["targets"][target] / "release" / ("materialize-cli" + suffix)
        shutil.copyfile(binary, bundle / "bin" / binary.name)
        (bundle / "bin" / binary.name).chmod(0o755)
        provenance["notice_files_missing"] = rust_notices(source, bundle)
        shutil.copyfile(source / "LICENSE", bundle / "notices" / "Materialize-LICENSE")
        shutil.copyfile(lock, bundle / "Cargo.lock")
        shutil.copyfile(lock_dir / "lock.json", bundle / "lock.json")
        provenance["lock_sha256"] = sha256(lock)
        provenance["tools_built"].append("materialize")
    shutil.copyfile(CONFIG, bundle / "sources.json")
    for patch in PATCHES:
        shutil.copyfile(patch, bundle / patch.name)
    # Smoke must succeed before an artifact is emitted. No GPU claim.
    from smoke import smoke
    provenance["smoke"] = smoke(bundle / "bin", work / "smoke" / target, not only_crunch)
    write_json(bundle / "metadata.json", provenance)
    output.mkdir(parents=True, exist_ok=True)
    label = "crunch-only" if only_crunch else "material-tools"
    bundle_zip(bundle, output / f"{label}-{target}.zip")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["prepare-lock", "build"])
    parser.add_argument("--work", type=Path, default=HERE.parents[1] / "dist" / "material-tools")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--target", choices=list(PINS["targets"]), default=native_target())
    parser.add_argument("--lock-dir", type=Path)
    parser.add_argument("--only-crunch", action="store_true")
    args = parser.parse_args()
    work = args.work.resolve()
    work.mkdir(parents=True, exist_ok=True)
    output = (args.output or work / ("lock" if args.command == "prepare-lock" else "artifacts")).resolve()
    if args.command == "prepare-lock":
        prepare_lock(work, output)
    else:
        build(work, output, args.target, args.lock_dir.resolve() if args.lock_dir else None, args.only_crunch)


if __name__ == "__main__":
    main()