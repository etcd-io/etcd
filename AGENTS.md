# etcd

## Constraints

- **Generated files are read-only.** Never hand-edit `*.pb.go`, `*.pb.gw.go`, or `scripts/etcd_version_annotations.txt`. Run `./scripts/genproto.sh`, then `./scripts/update_proto_annotations.sh`.
- **New proto fields need a version.** Annotate new fields, messages, and enum values under `api/` with `etcd_version_*` set to the next minor release in `api/version/version.go`.
- **Apply must be deterministic.** Code in `server/etcdserver/apply` and `server/etcdserver/txn` must give the same result on every member. Generate IDs, tokens, and timestamps before proposing.
- **Don't wrap errors returned to clients.** `server/etcdserver/api/v3rpc` maps errors by identity, so a `%w`-wrapped error becomes `codes.Unknown`.
- **Respect module boundaries.** Each module's `.gomodguard.yaml` lists imports it must not use. Never import `go.etcd.io/etcd/tests/v3` from a library module.
- **Some example tests are symlinks.** The `example*_test.go` files under `client/v3/` point into `tests/integration/clientv3/`. Edit the targets, never replace the links.

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
ETCD_VERIFY=all go test ./server/storage/mvcc -run 'TestKVPutMultipleTimes$'  # unit test, with invariant checks
go test -tags=integration ./tests/common -run 'TestKVPut$'                     # common test, in-process cluster
go test -tags=e2e ./tests/common -run 'TestKVPut$'                             # common test, binaries in bin/
```

- e2e tests use the binaries in `bin/`. Run `make build` after changing code.
- Failpoint tests skip unless built with `make gofail-enable`. A SKIP means the test did not run. Run `make gofail-disable` before committing.

## Style

- Comments explain "why", not "what". Do not add comments when the code itself is easily understood.
- Follow the patterns in neighboring code. Extend existing tests and helpers instead of adding new ones.
- Tests: use testify `require` (the `f` variants when passing a message), `zaptest.NewLogger(t)`, and `require.Eventually` instead of `time.Sleep`. No `t.Parallel()` in integration or e2e tests.
- Logging: zap with kebab-case keys, for example `zap.String("local-member-id", id.String())`.
