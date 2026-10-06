# UI source discovery v1

The CLI discovers `.kui` components recursively under `ui/` and `assets/ui/`.
Build-time layouts under `ui/layouts/` use the same extension and are excluded
from component discovery. Explicit asset overrides use the same compiler.
Serialized presentation assets remain separate from authored components.

KartUI owns SFC parsing, indented styles, lowering, schema and Go generation.
The CLI owns discovery, confined source reads, staging, encoding and reachability.
Every SFC uses the client/package adapter, including files without a script.
Unused generated components do not retain their own assets; dynamic lookups retain
conservative behavior. Generated Go belongs in derived staging and `.karty/ui`.

Legacy declarations, `.ui` authoring discovery and engine view adapter generation
have been removed. Samples and docs use template-first SFCs. This changes no wire
or runtime schema. Released SDK snapshots remain immutable; new UI template
scaffolding uses SDK 0.0.9's SFC templates. Released game templates still scaffold.

Tests cover SFC discovery, sparse overrides, staging, embedding and confined
reads. Candidate integration tests exercise SFC template rendering and unused
component stripping with `KARTY_TEST_SDK=0.0.9`. Standalone checks do not require
private engine source or an unpublished candidate SDK.
