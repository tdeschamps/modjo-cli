# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-06-25

### Added

- Full Modjo Public API v2 parity — 19 new endpoints across the CLI:
  - `calls upload` (upload a recording by URL), `calls notes`,
    `calls next-steps`, `calls crm-answers`, and a `calls tags`
    sub-group (`list`/`add`/`remove`).
  - `deals summary` (AI-generated deal summary).
  - `teams create`/`update`/`delete`/`members` — full team management.
  - `users update`, `users add-team`, `users remove-team` — user edits and
    team membership.
  - `webhooks update`.
  - `crm-templates list`/`get`/`fields` — CRM filling templates and fields.
- `tags`, `topics`, and `webhooks` commands.
- `MODJO_NO_KEYRING` env toggle to force the file-backed credential store and
  skip the OS keychain (useful in CI, headless shells, and on macOS where the
  keychain can prompt).
- Bold section headings in `--help` output when color is enabled.
- Bundled the full machine-readable OpenAPI v2 spec at
  `docs/modjo-openapi.full.json` alongside the curated reference digest.

### Changed

- Aligned the entire client with the Modjo OpenAPI v2 spec: page-based
  pagination (`page`/`size` with a `{data, pagination}` envelope), real filter
  parameters (numeric `account_id`/`deal_id`/`user_id`, `from`/`to` dates,
  single `--status`), and models matching the published schemas.
- Partial-update commands (`users update`, `webhooks update`) now send only the
  flags you set, so passing an explicit empty value (e.g. `--phone ""`) clears
  the field instead of being silently dropped.
- `modjo ask` prints the human-readable answer by default and emits JSON only
  when explicitly requested (`--json`, `-o <non-table>`, or `--jq`).
- Redesigned the `modjo info` logo as the Modjo waveform mark.

### Fixed

- Credential validation now calls a real endpoint (the API has no `/me`
  route), fixing a spurious 404 on login.
- A credential written via `MODJO_NO_KEYRING` is now found when reading in
  normal keychain mode (cross-store fallback).
- Pagination no longer stops short on a short or zero-`size` page; the
  iterator now stops on cumulative total coverage. Auto-pagination also bounds
  a runaway server (one that never signals the end) with an explicit error
  instead of looping unboundedly, and the short-page stop compares against the
  size the server actually served rather than the requested page size, so a
  server paging below the requested size no longer truncates the results.
- CRM filling field `isActive`/`isAutoPush` now serialize as `false` in
  `-o json` instead of being dropped, so consumers can tell "inactive" from
  "absent".
- `calls summary` and `calls next-steps` emit full answers/descriptions in
  CSV/TSV instead of the ellipsized form meant only for the interactive table.
- `teams members` now renders the same shape as `users get` for the same
  person (`name`/`title`/`department`), not the raw `firstName`/`lastName`/
  `job*` fields.
- `modjo ask` answers are unwrapped from the MCP `{"answer":"..."}` envelope
  instead of being shown as a raw JSON blob.

### Removed

- `emails` and `agents` commands and the `ask --agent` flag — the public API
  exposes neither.
- `deals get` and `deals open` — the API exposes no `GET /deals/{id}` (only
  `GET /deals` and `GET /deals/{id}/summary`), so both returned 404. Use
  `deals list` to inspect deals and `deals summary` for a single deal.
