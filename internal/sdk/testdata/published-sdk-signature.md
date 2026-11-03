# Published signature fixture

These public SDK 0.0.19 release bytes exercise the CLI's actual pinned Ed25519
verifier without exposing or synthesizing its private signing key.

- [Manifest](https://github.com/karty-game/karty-sdk/releases/download/sdk-v0.0.19/sdk-0.0.19.toml), SHA-256 `c5283c83f469dd53693a51b51f2ab5200b96466ea3de0353ccd224667e8dc4c2`.
- [Detached signature](https://github.com/karty-game/karty-sdk/releases/download/sdk-v0.0.19/sdk-0.0.19.toml.sig), SHA-256 `76f1ec9a3abbb046e79338d5beffbe06557c834dc7221299c7176024a1c76517`.

The signature is stored as base64 text; decode it to its original 64 bytes before
verification. Tests read these committed fixtures without network access.
