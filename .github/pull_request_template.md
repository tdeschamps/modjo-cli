## What & why

<!-- Briefly describe the change and the motivation. -->

## Which test drove this change?

<!-- Required. Name the failing test you wrote first (Red → Green → Refactor).
     e.g. internal/api: TestDealsList_FollowsPagination -->

## Checklist

- [ ] `make test` passes (`go test -race ./...`)
- [ ] `make lint` / `gofmt` clean
- [ ] JSON output changes (if any) are intentional and covered by tests
- [ ] Docs/`--help` updated if the user-facing surface changed
