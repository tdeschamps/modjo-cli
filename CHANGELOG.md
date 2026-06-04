# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `tags`, `topics`, and `webhooks` commands.
- `MODJO_NO_KEYRING` env toggle to force the file-backed credential store and
  skip the OS keychain (useful in CI, headless shells, and on macOS where the
  keychain can prompt).
- Bold section headings in `--help` output when color is enabled.

### Changed

- Aligned the entire client with the Modjo OpenAPI v2 spec: page-based
  pagination (`page`/`size` with a `{data, pagination}` envelope), real filter
  parameters (numeric `account_id`/`deal_id`/`user_id`, `from`/`to` dates,
  single `--status`), and models matching the published schemas.
- `modjo ask` prints the human-readable answer by default and emits JSON only
  when explicitly requested (`--json`, `-o <non-table>`, or `--jq`).
- Redesigned the `modjo info` logo as the Modjo waveform mark.

### Fixed

- Credential validation now calls a real endpoint (the API has no `/me`
  route), fixing a spurious 404 on login.
- A credential written via `MODJO_NO_KEYRING` is now found when reading in
  normal keychain mode (cross-store fallback).
- Pagination no longer stops short on a short or zero-`size` page; the
  iterator now stops on cumulative total coverage.
- `modjo ask` answers are unwrapped from the MCP `{"answer":"..."}` envelope
  instead of being shown as a raw JSON blob.

### Removed

- `emails` and `agents` commands and the `ask --agent` flag — the public API
  exposes neither.
