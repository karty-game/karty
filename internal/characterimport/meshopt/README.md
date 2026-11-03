# Meshopt decoder

The MIT-licensed source is vendored unchanged from
`https://github.com/zeux/meshoptimizer/blob/v0.22/js/meshopt_decoder.module.js`.
Its SHA-256 is pinned in `gen.go`; the license is retained beside it.

`go generate` extracts the upstream baseline WASM bytes without executing JS or
requiring a network connection. The CLI executes that trusted module with a
bounded pure-Go WASM runtime during GLB preparation. CLI users need neither
Node nor a native C library. No source model decoder runs during gameplay.
