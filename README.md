<div align="center">

# modjo

**One CLI for the Modjo REST API v2 and the Modjo MCP server — scriptable, agent-friendly, and at home in any terminal.**

[![CI](https://github.com/tdeschamps/modjo-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/tdeschamps/modjo-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tdeschamps/modjo-cli.svg)](https://pkg.go.dev/github.com/tdeschamps/modjo-cli)
[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Coverage](https://img.shields.io/badge/coverage-95%25-brightgreen)](#development)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-fe5196.svg)](https://www.conventionalcommits.org)

<sub>Go Report Card, codecov, and the release/version badges activate once the repository is public.</sub>

</div>

---

`modjo` wraps the **Modjo REST API v2** and the **Modjo MCP server** behind one
consistent interface. It serves three audiences from a single static binary:

- **Developers / integrators** — script exports, pipe JSON, wire CI jobs.
- **Sales Ops / RevOps** — pull pipeline, call, and deal reports without code.
- **AI / agent power users** — run `modjo ask` natural-language queries and
  expose `modjo mcp serve` so any MCP client (Claude, Cursor, Codex) can drive
  Modjo through the CLI.

Design north star: feel like `gh`, `stripe`, and `sentry-cli` — predictable
noun-verb commands, great `--help`, machine-readable output in pipes,
human-readable in a TTY, and first-class auth.

## Table of contents

- [Install](#install)
- [Quickstart](#quickstart)
- [Commands](#commands)
- [Output & scripting](#output--scripting)
- [Authentication](#authentication)
- [Configuration](#configuration)
- [MCP integration](#mcp-integration)
- [Exit codes](#exit-codes)
- [Development](#development)
- [License](#license)

## Install

```sh
# Homebrew (macOS/Linux)
brew install tdeschamps/tap/modjo

# Shell installer (downloads the latest signed release binary)
curl -fsSL https://raw.githubusercontent.com/tdeschamps/modjo-cli/main/install.sh | sh

# From source (Go 1.24+)
go install github.com/tdeschamps/modjo-cli/cmd/modjo@latest
```

Prebuilt, signed binaries for `darwin/{amd64,arm64}`, `linux/{amd64,arm64}`, and
`windows/amd64` are attached to every [release](https://github.com/tdeschamps/modjo-cli/releases).

> **Coming soon:** Scoop/winget on Windows and a Docker image. These ship once
> the bucket repo and container registry are set up — see the disabled blocks
> in `.goreleaser.yaml`.

## Quickstart

```sh
# 1. Authenticate (paste a key interactively, or pipe it in CI)
modjo auth login --web
echo "$MODJO_KEY" | modjo auth login --with-token
modjo auth status

# 2. Pull this week's calls for an account, as CSV
modjo accounts list --name "Contoso" --json --jq '.[0].crmId'   # -> 001ABC
modjo calls list --account 001ABC --since 7d --all -o csv > contoso_calls.csv

# 3. Ask the AI about a deal
modjo deals list --account 001ABC --status open --json --jq '.[0].crmId'   # -> 006XYZ
modjo ask deal 006XYZ "What are the risks and the single best next step?"

# 4. Wire the CLI as an MCP server for Claude Desktop
modjo mcp config --client claude-desktop      # paste into claude_desktop_config.json
modjo mcp serve

# 5. Raw escape hatch — anything the typed commands don't cover yet
modjo api GET /calls --param "limit=50" --param "relations=summary" --paginate
```

## Commands

```
modjo
├── auth        login | logout | status | refresh | switch | token
├── config      get | set | list | edit
├── profiles    list | use
├── calls       list | get | transcript | summary | export | open
├── deals       list | get | open
├── accounts    list | get | open
├── contacts    list | get
├── users       list | get | create | delete
├── teams       list | get
├── tags        list
├── topics      list
├── webhooks    list | get | create | delete
├── ask         call | deal | account   (natural language over the MCP)
├── mcp         serve | tools | call | config
├── api         raw authenticated request escape hatch
├── info        version, configuration & status (logo banner)
├── doctor      connectivity & credential diagnostics
├── completion  bash | zsh | fish | powershell
├── docs        open the docs
├── update      self-update
└── version
```

Every `list` command shares the same contract: filter flags (`--account`,
`--status`, `--from`, `--to`, …), `--limit`/`--all` pagination, and
`get` accepts one or more IDs.

## Output & scripting

- **TTY** → colorized, aligned tables. **Piped/redirected** → JSON.
- Override anytime with `-o table|json|csv|tsv|yaml` or the `--json` shorthand.
- Built-in **`--jq`** filter (powered by [gojq](https://github.com/itchyny/gojq);
  no external `jq` needed): `modjo deals list --json --jq '.[] | {name, amount}'`.
- `--columns name,amount` to pick/order columns for tables and CSV.
- Stable [exit codes](#exit-codes) for robust scripts.

```console
$ modjo deals list --status open --limit 2
CRMID  NAME                ACCOUNT  STATUS  AMOUNT     CLOSE       SOURCE
D1     Contoso – Platform  Contoso  Open    EUR 42000  2026-06-18  Inbound
D2     Globex – Renewal    Globex   Open    EUR 18500  2026-06-25  Outbound

$ modjo deals list --status open --json --jq '.[].amount' | paste -sd+ | bc
60500
```

## Authentication

`modjo` resolves the token in this order: `--api-key`/`--token` flag →
`MODJO_API_KEY`/`MODJO_TOKEN` env → stored credential for the active profile.
Secrets are stored in the **OS keychain** when available (macOS Keychain,
Windows Credential Manager, libsecret/kwallet), with a `0600` file fallback.
Set **`MODJO_NO_KEYRING`** to skip the keychain entirely and persist to the
`0600` file — handy in CI, headless shells, or on macOS where the keychain can
pop an interactive prompt. The file path is the same either way, so toggling it
on or off keeps reading the same stored credential.
Keys are never printed back — `modjo auth status` shows only a masked
fingerprint. OAuth (device + PKCE) flows are spec'd and ready to enable when
Modjo ships public OAuth clients.

## Configuration

Config lives at `~/.config/modjo/config.toml` (XDG-respecting;
`%APPDATA%\modjo\config.toml` on Windows). Any setting resolves
**flag → env var → profile → built-in default**.

```toml
active_profile = "default"

[profiles.default]
workspace     = "acme-eu"
base_url      = "https://api.modjo.ai/v2"
output        = "table"
default_limit = 50
```

Manage it with `modjo config get/set/list/edit` and `modjo profiles list/use`.
Relevant env vars: `MODJO_API_KEY`, `MODJO_TOKEN`, `MODJO_PROFILE`,
`MODJO_BASE_URL`, `MODJO_MCP_URL`, `MODJO_OUTPUT`, `MODJO_NO_COLOR`,
`MODJO_LANGUAGE`, `MODJO_DEBUG`, `MODJO_NO_KEYRING`.

## MCP integration

`modjo mcp serve` runs a local MCP server that re-exposes the Modjo tools,
authenticating upstream with your stored credential — so MCP clients never touch
the raw key and need no `mcp-remote` shim. `modjo mcp config --client cursor`
prints a ready-to-paste snippet; `modjo mcp tools` and `modjo mcp call` are there
for inspection and one-off scripted tool calls.

## Claude Code / agents

A Claude Code **skill** ships with this repo so agents drive the CLI efficiently
(correct flags, the numeric-id-vs-name filter gotcha, `--json`/`--jq` idioms).
It loads automatically for anyone using Claude Code inside this checkout.

To get it in **your own** projects, install it as a plugin:

```sh
/plugin marketplace add tdeschamps/modjo-cli
/plugin install modjo-cli@modjo
```

For read-heavy agent workflows, wiring `modjo mcp serve` as an MCP server (see
above) gives native typed tools and is even more reliable than the CLI skill.

## Exit codes

| Code | Meaning              | Code | Meaning                 |
| ---- | -------------------- | ---- | ----------------------- |
| `0`  | success              | `5`  | not found (404)         |
| `1`  | generic error        | `6`  | rate limited (429)      |
| `2`  | usage / bad flags    | `7`  | validation (422)        |
| `3`  | auth required (401)  | `8`  | upstream / server (5xx) |
| `4`  | forbidden (403)      | `124`| operation timed out     |

## Terminal experience

On an interactive terminal `modjo` adds a few niceties — and every one of them
is **suppressed the moment output isn't a TTY**, so pipes, `--json`, and scripts
stay byte-for-byte plain:

- **`modjo info`** — a logo banner with your version, profile, endpoints, and
  auth status (and `--json` for a machine-readable summary).
- **Spinners** on the slow paths (`ask`, `auth login`, `doctor`) and a live
  **`Fetched N …` count** during `--all` sweeps — all on stderr, never stdout.
- **Colors & links** — status-colored deal stages and OSC 8 hyperlinks on
  resource names (click to open the CRM record in supporting terminals).
- **Success banners** with next steps (e.g. after `auth login`).
- **Guided `modjo ask`** — run it with no arguments to pick an entity, resolve
  it by name, and ask, interactively.
- **Update notices** — a non-blocking "new version available" hint, suppressible
  with `MODJO_NO_UPDATE_NOTIFIER=1`.

Turn the chrome off anytime with `--no-color`, `--quiet`, `--hide-spinner`, or
the standard `NO_COLOR`.

## Development

Built **test-first** in Go behind interface seams (see
[`ARCHITECTURE.md`](ARCHITECTURE.md) and [`CONTRIBUTING.md`](CONTRIBUTING.md)).

```sh
make test       # go test -race ./...
make cover      # coverage profile + total (fails under the gate)
make lint       # golangci-lint
make fmt        # gofumpt + goimports
make build      # ./bin/modjo
```

The CI gate runs `gofumpt`, `golangci-lint`, `go vet`, `govulncheck`, the full
race-enabled test suite (unit + `testscript` e2e), and a cross-compile matrix.

## License

[MIT](LICENSE) © Thomas Descamps
