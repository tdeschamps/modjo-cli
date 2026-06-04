# Architecture

This document explains how `modjo` is structured so you know where new code
goes. It mirrors §3–§4 of the technical spec.

## Layers

Dependencies point **downward only**. The `cmd` layer never builds HTTP
requests directly — it calls a service/adapter behind an interface, and those
interfaces are the test seams.

```
 cmd layer (Cobra commands)        thin: parse flags, call adapters, render
 ─────────────────────────────────────────────────────────────────────────
 api (REST v2) │ mcp (client+srv) │ output (renderers)   adapters
 ─────────────────────────────────────────────────────────────────────────
 platform: config, auth/keychain, iostreams, httpclient (RoundTripper chain)
```

## Package map

| Package | Responsibility |
|---|---|
| `cmd/modjo` | Entrypoint: build root command, execute, map errors → exit codes. Hosts the `testscript` e2e harness. |
| `internal/cmd/*` | One package per command group. Thin; depends on `cmdutil` + adapters. |
| `internal/cmdutil` | The `Factory` (dependency injection), global flags, exit-code mapping, render/prompt/browser helpers. |
| `internal/api` | REST v2 adapter: typed models, filters, auto-paginating iterators (`iter.Seq2`). |
| `internal/mcp` | MCP JSON-RPC client (`ask`, `tools`, `call`) and the embedded stdio server (`mcp serve`). |
| `internal/config` | TOML config, profiles, precedence resolution (flag → env → profile → default). |
| `internal/auth` | Credential modeling and storage (OS keychain with a 0600 file fallback). |
| `internal/output` | Renderers (table/json/csv/tsv/yaml) and the built-in `--jq` filter. |
| `internal/iostreams` | Testable stdin/stdout/stderr with TTY/color detection. |
| `internal/httpclient` | The `RoundTripper` chain: auth → retry → logging. |
| `internal/text` | Pure helpers (date normalization). |

## Key seams (interfaces)

- HTTP goes through an injectable base `http.RoundTripper` so tests swap in a fake.
- The REST client's list methods return `iter.Seq2[T, error]` and auto-follow `nextCursor`.
- `auth.Store` abstracts credential persistence (`MemoryStore` in tests).
- `output.Printer` writes to an `io.Writer` so tests assert on rendered bytes.
- The `text.Clock` is injected so relative-date math is deterministic.

## Adding a resource command (recipe)

1. Add the model + filter + client method in `internal/api` (with an httptest test).
2. Create `internal/cmd/<group>` exposing `NewCmd<Group>(f *cmdutil.Factory)`.
3. Define the `output.Field` list and call `cmdutil.CollectAndRender` / `RenderSlice`.
4. Register it in `internal/cmd/root`.
5. Add a `.txtar` script under `test/script` for the user-facing contract.
