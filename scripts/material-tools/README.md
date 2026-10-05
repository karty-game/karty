# Native material-tool artifacts

Developer build prerequisite only. No CLI/SDK adapter, download contract, release
URL, publication, commit or tag is created. The manual **Material tools** workflow
builds native Linux amd64/arm64, macOS arm64 and Windows amd64 artifacts. Linux
arm64 uses `ubuntu-24.04-arm`; macOS uses native `macos-14`; Windows uses MSVC x64
on `windows-2022`. The script rejects a mismatched native architecture.

## Inputs and replay

- [sources.json](sources.json) pins ChevyRay/crunch at
  `a836f6dbaf2af6bd10e1f1bff0c74ad82ceaa913` and **AiGameKit/Materialize** at
  `1a3fe7d052a464d3217ef66d2bce9889d3946792`. Source archives are SHA256-checked.
  There are no upstream binary assets involved. Materialize is the Rust/wgpu CLI,
  not the Unity application or the separate repository named in its manifest.
- Rust **1.99.0** was verified against the stable channel manifest dated
  2026-10-01 (rustc commit `b940084d7eb6a299eb4bfeb8e34901bc051e7ac4`). The pinned
  source declares Rust 1.87 minimum, edition 2024 and wgpu 30. Use rustup with
  the exact pinned version; the script never installs a toolchain itself.
- Upstream has no committed Cargo.lock. The workflow's first job resolves once
  with that toolchain, uploads **material-tools-lock**, then each native job
  validates the shared lock/source/manifest/toolchain identity and runs Cargo
  `build --release --locked --bin materialize-cli --target <native-triple>`.
  Fetch and metadata collection also use `--locked`. The lock artifact contains
  Cargo.lock, source pins and lock provenance. Save it beyond the 90-day artifact
  retention if needed; a new dispatch may resolve different dependency versions.
- Replay with the preserved lock directory, not a fresh resolution. The script
  accepts `prepare-lock --output <directory>` or `build --lock-dir <directory>
  --target <target>` through Python. Default generated work is ignored repository
  `dist/material-tools`. Python 3.10+, git, a native C++ compiler and rustup/Cargo
  are prerequisites. CI installs only the exact Rust toolchain via rustup.

## Crunch changes and checks

[crunch-order.patch](crunch-order.patch) keeps ascending area sorting but orders
equal-area names descending in the vector, so `pop_back()` packs names ascending.
Names must be unique; duplicate-name validation and rotation/trim/premultiply/
deduplication policy belong to the future adapter, not these build scripts.
[crunch-msvc.patch](crunch-msvc.patch) removes an invalid Windows `PathToStr`
conversion of an already UTF-8 `string`. Both patches are applied only to generated
source copies, with `git apply --check` first. `cstring` is force-included for
upstream's undeclared C memory functions. Windows uses Unicode defines, C++14,
static MSVC CRT and `/Brepro`; Unix compilers use C++11.

[smoke.py](smoke.py) makes real PNG fixtures, packs two reversed equal-area input
orders, and requires identical atlas PNG/JSON plus ascending names and 4x4 geometry.
It omits rotation, trim, deduplication and premultiplication. Materialize smoke
executes help, version and list-maps only: **no GPU computation is validated**.
For a C++-only local check, `build --only-crunch` emits a clearly named partial
artifact, never a full material-tools bundle. Unit tests run with Python's unittest
discovery in this directory and require no third-party Python packages.

## Bundle and reproducibility boundary

Each successful full build produces a ZIP and its SHA256 sidecar containing:

- Native `crunch` and `materialize-cli` executables (Windows `.exe`).
- Source pins, patch files/hashes, shared Cargo.lock/provenance, toolchain/compiler
  and runner identity, smoke results, and per-file SHA256SUMS.
- Crunch source files retaining embedded LodePNG, tinydir and bin-packer notices;
  Materialize's MIT license; resolved Rust dependency identities/SPDX expressions
  and available LICENSE/COPYING/COPYRIGHT/NOTICE files. Metadata explicitly lists
  dependencies without such notice files for review before any future publication.

ZIP ordering, timestamps and permissions are normalized. This is repeatable source
and dependency selection with auditable outputs, **not a claim of byte-identical
binaries**: runner images, native C++ compilers/system SDKs and build paths are not
immutable. Linux binaries inherit their runner's libc baseline; macOS deployment
compatibility, dynamic-library availability and GPU backends require separate
runtime validation. Cargo registry sources use the preserved lock checksums.
Nothing here implements WASM execution, browser graphics, GPU-output determinism,
or automatic publication. Failed build/smoke jobs do not emit a tool bundle.