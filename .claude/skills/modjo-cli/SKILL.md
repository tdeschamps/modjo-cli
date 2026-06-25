---
name: modjo-cli
description: Use when querying or managing Modjo data (calls, deals, accounts, contacts, users, teams, webhooks, CRM templates) or asking AI questions about a call/deal/account via the `modjo` CLI. Covers the correct flags, the numeric-ID-vs-name gotcha, and efficient --json/--jq idioms so commands work first try.
---

# Driving the `modjo` CLI efficiently

`modjo` wraps the Modjo REST API v2 and MCP server. Output is a human table on a
TTY and **JSON when piped**, so in scripts/agents you almost always want `--json`.

## The three rules that prevent most mistakes

1. **List/get filters take NUMERIC ids, not CRM ids.** `--account`, `--deal`,
   `--user` want the API's numeric `id` (e.g. `4551`), not a Salesforce CRM id
   (`001ABC…`). Resolve a name → numeric id first (see Recipes).
2. **Always pipe through `--json` (+ `--jq`) for a single value.** Don't parse
   tables. `modjo deals list --json --jq '.[0].id'` beats reading a table.
3. **`ask` is for AI questions; `list`/`get` are for data.** Use `modjo ask
   call|deal|account <id> "<question>"` for natural-language analysis. Use
   `list`/`get`/`api` to fetch records.

## Auth (do this once)

Token resolves: `--api-key`/`--token` flag → `MODJO_API_KEY`/`MODJO_TOKEN` env →
stored credential. Quickest for scripting:

```sh
export MODJO_API_KEY=mjo_live_xxx
modjo doctor          # verify connectivity + credentials
```

If credentials were saved with `MODJO_NO_KEYRING=1`, keep that env var set so
later commands read the same (file) store. `modjo info --json` shows the
resolved profile, base URL, MCP URL, and auth status.

## Commands and their real flags

| Command | Key flags |
| --- | --- |
| `calls list` | `--account`/`--deal`/`--user` (numeric), `--since`/`--until` (YYYY-MM-DD or `30d`), `--expand contacts,deal,account,users` |
| `calls get <id>` | `--expand …` |
| `calls transcript <id>` | `--speakers`, `--timestamps` |
| `calls summary <id>` | (pre-generated AI summaries) |
| `calls notes <id>` / `calls next-steps <id>` / `calls crm-answers <id>` | per-call AI notes, next steps, and CRM answers |
| `calls tags list\|add\|remove <id>` | `add` takes `--tag <tagId>`; `remove <id> <tagId>` |
| `calls upload` | `--media-url <url> --date <date> --participant <email:type[:name]>` (repeatable) |
| `deals list` | `--name`, `--account` (numeric), `--status open\|won\|lost\|closed` |
| `deals summary <id>` | AI-generated deal summary |
| `accounts list` | `--name` |
| `contacts list` | `--name` |
| `users list` | `--email` (exact match only) |
| `users create` | `--first-name --last-name --email` required; optional `--role --job-title --job-department --phone --timezone` |
| `users update <id>` | partial update — only the flags you set are sent (pass `--phone ""` to clear) |
| `users add-team <id> --team <teamId>` / `users remove-team <id> <teamId>` | team membership |
| `teams create --name <name>` / `teams update <id> --name <name>` / `teams delete <id>` / `teams members <id>` | full team management |
| `webhooks update <uuid>` | partial update — only the flags you set are sent |
| `crm-templates list` / `crm-templates get <uuid>…` / `crm-templates fields <uuid>` | CRM filling templates and their fields |
| `tags list` / `topics list` / `teams list` / `webhooks list` | paging only |
| `ask call\|deal\|account <id> "<q>"` | `--language`; prints prose, add `--json` for structured |
| `api <METHOD> <path>` | `--param k=v` (repeatable), `--field k=v` (body), `--paginate`, `--input -` |

**Global flags worth knowing:** `--json` (force JSON), `--jq '<filter>'`
(built-in gojq, no external jq), `--limit N`, `--all` (auto-paginate every
page), `--columns a,b`, `--profile <name>`, `-o table|json|csv|tsv|yaml`.

## Recipes (copy these patterns)

Resolve a name to the numeric id you need for filtering:
```sh
# account name -> numeric id
acct=$(modjo accounts list --name "Contoso" --json --jq '.[0].id')
modjo calls list --account "$acct" --since 30d --json
```

Pull one field instead of a whole table:
```sh
modjo deals list --status open --json --jq '.[] | {id, name, amount}'
modjo calls list --json --jq '.[0].id'
```

Export every page (not just the default limit) to CSV:
```sh
modjo calls list --account "$acct" --all -o csv > calls.csv
```

Ask an AI question about a specific record (find the id first):
```sh
call=$(modjo calls list --account "$acct" --json --jq '.[0].id')
modjo ask call "$call" "What are the risks and the single best next step?"
# add --json to get {answer, entity, type}
```

Raw escape hatch for anything the typed commands don't cover:
```sh
modjo api GET /calls --param "size=50" --param "expand=deal,account" --paginate
```

## Pitfalls (what bit Claude before)

- **Use the flag tables above instead of scraping `--help`.** A common failure
  is burning turns trying to `awk`/`grep` the flags out of `modjo <cmd> --help`
  in a pipeline and getting empty results from a quoting/anchor mistake. The
  flags you need are in this skill. If you must confirm, run `modjo <cmd>
  --help` and **read the plain output directly** — it's clean UTF-8 on stdout,
  no ANSI codes, no CRLF, so it needs no post-processing.
- **Don't guess flags.** There is no `--contact` on `calls list`, no
  `--amount-min` on `deals list`, no `--name`/`--role` on `users list` (only
  `--email`). When unsure, run `modjo <cmd> --help` once — it's accurate.
- **Don't pass a CRM id to `--account`/`--deal`/`--user`** — they want numeric
  ids. Resolve via a `list --name … --json --jq '.[0].id'` first.
- **Don't read tables to extract a value** — pipe `--json --jq`.
- **`users list` can't search by name** — only exact `--email`. To find a user
  by name, `modjo users list --json --jq '.[] | select(.name|test("…";"i"))'`.
- **There is no `emails` or `agents` command** (the public API has neither).
- Exit codes are stable: `3` auth, `4` forbidden, `5` not-found, `6` rate
  limited, `7` validation, `8` upstream, `124` timeout — branch on these.

## Targeting an environment

Endpoints resolve flag → env → profile → default. To hit staging:
`modjo --profile modjo-internal …`, or set `MODJO_BASE_URL`/`MODJO_MCP_URL`.
`modjo info` shows what's currently resolved.
