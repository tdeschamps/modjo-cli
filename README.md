# modjo

`modjo` is a single command-line tool that wraps the **Modjo REST API v2** and
the **Modjo MCP server** behind one consistent, scriptable, agent-friendly
interface. It feels like `gh`/`stripe`: predictable noun-verb commands, great
`--help`, machine-readable output in pipes, human-readable in a terminal, and
first-class auth.

## Install

```sh
# Homebrew (macOS/Linux)
brew install modjo/tap/modjo

# Shell installer
curl -fsSL https://cli.modjo.ai/install.sh | sh

# From source
go install github.com/tdeschamps/modjo-cli/cmd/modjo@latest
```

## Quickstart

```sh
# Authenticate (paste a key, or pipe it for CI)
modjo auth login --web
echo "$MODJO_KEY" | modjo auth login --with-token
modjo auth status

# Pull this week's calls for an account, as CSV
modjo accounts list --name "Contoso" --json --jq '.[0].crmId'   # -> 001ABC
modjo calls list --account 001ABC --since 7d --all -o csv > calls.csv

# Ask the AI about a deal
modjo ask deal 006XYZ "What are the risks and the single best next step?" --agent DealBriefing

# Wire the CLI as an MCP server for Claude Desktop
modjo mcp config --client claude-desktop
modjo mcp serve

# Raw escape hatch — anything the typed commands don't cover
modjo api GET /calls --param "limit=50" --param "relations=summary" --paginate
```

## Output & scripting

- TTY → colorized tables. Piped/redirected → JSON. Override with
  `-o table|json|csv|tsv|yaml` or `--json`.
- Built-in `--jq` filter (no external `jq` needed).
- `--columns a,b,c` to pick/order columns; `--limit`/`--all` for pagination.
- Stable [exit codes](#exit-codes) for scripting.

## Configuration

Config lives at `~/.config/modjo/config.toml` (XDG-respecting). Settings resolve
**flag → env → profile → built-in default**. Manage it with `modjo config` and
`modjo profiles`. Useful env vars: `MODJO_API_KEY`, `MODJO_PROFILE`,
`MODJO_BASE_URL`, `MODJO_MCP_URL`, `MODJO_OUTPUT`, `MODJO_NO_COLOR`.

## Exit codes

| Code | Meaning | | Code | Meaning |
|---|---|---|---|---|
| 0 | success | | 5 | not found (404) |
| 1 | generic error | | 6 | rate limited (429) |
| 2 | usage / bad flags | | 7 | validation (422) |
| 3 | auth required (401) | | 8 | upstream/server (5xx) |
| 4 | forbidden (403) | | 124 | timed out |

## Development

Built test-first in Go. See `CONTRIBUTING.md` and `ARCHITECTURE.md`.

```sh
make test    # go test -race ./...
make build   # ./bin/modjo
make lint
```
