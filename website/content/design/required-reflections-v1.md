# Required offline reflections v1

Levels that rely on saved cubemaps declare the build requirement in `level.toml`:

```toml
[reflections]
required = true
```

This is a CLI build policy; it adds no runtime work or wire format. The default
remains optional for existing projects. A required level must use fixed offline
lighting and contain a saved reflection capture matching its compiled input
digest. Missing, stale, corrupt or empty captures fail packaging with an
instruction to run `karty render reflections LEVEL`. Capture preparation itself
bypasses this requirement so a fresh checkout can create the first capture.

Required captures also match the complete offline reflection plan: every probe
ID, origin and bounds, plus every sector and solid assignment. Structural source
validation alone permits a partial image set, which cannot fulfill this policy.

The sample builder bakes lighting, captures every required level, then packages
the web game. Hangar declares this requirement. Cached `.karty/bakes` files can
accelerate preparation but cannot satisfy a different compiled input digest.

Local sample captures use hardware by default. Hosted sample CI explicitly sets
`KARTY_SAMPLE_REFLECTION_BACKEND=software` to generate the offline assets on Mesa;
these CI captures do not establish hardware visual quality or device performance.
There is no automatic backend fallback. Any capture failure stops publication.
