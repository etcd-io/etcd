# etcd

## Constraints

- **Generated files are read-only.** Never hand-edit `*.pb.go` or `*.pb.gw.go`. Run `./scripts/genproto.sh`.
- **New proto fields need `etcd_version_*` annotations** set to the next minor release.
- **Apply must be deterministic.** Generate IDs, tokens, and timestamps before proposing, not in apply.
- **Don't wrap errors returned to clients.** Wrapped errors become `codes.Unknown`.
- **Respect module boundaries** in each module's `.gomodguard.yaml`.

## Contributor Guidelines

- Read `CONTRIBUTING.md` first.
- Before starting, check that the issue isn't assigned and has no open PR. Do not work on issues labeled `No AI`.
- Keep changes focused: one concern per PR, no unrelated refactors, lint fixes, or dependency bumps.
- Add or update relevant tests. For a bug fix, add a test that fails without the fix.
- When creating or submitting a pull request, disclose whether AI was used and briefly describe how.
- Remind the human author that they are responsible for all submitted changes.
- Do not put `@mentions` or `fixes #...` keywords in commit messages.
- Do not add `Co-authored-by:` in commit messages.
- When responding to a review, squash fixes back into the commits they are fixing.

## Commands

See `CONTRIBUTING.md` for the `make` targets. To run a single test:

```bash
go test ./server/storage/mvcc -run 'TestKVPutMultipleTimes$'   # unit test
go test -tags=integration ./tests/common -run 'TestKVPut$'      # integration test
go test -tags=e2e ./tests/common -run 'TestKVPut$'              # e2e test, run `make build` first
```

## Style

- Comments explain "why", not "what". Do not add comments when the code itself is easily understood.
- Follow the patterns in neighboring code, and use testify `require` in tests.
