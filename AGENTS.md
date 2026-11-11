# AGENTS.md

Instructions for AI coding agents working in this repository. They come from [CONTRIBUTING.md](CONTRIBUTING.md), the [contributor guide](Documentation/contributor-guide/), the AI guidance linked from the [pull request template](.github/PULL_REQUEST_TEMPLATE.md), and recurring maintainer review feedback. If this file disagrees with CONTRIBUTING.md or the contributor guide, those documents win.

## Before writing any code

- Every pull request must be linked to a GitHub issue. Confirm the issue is not assigned to someone else, has no open pull request, and is not already fixed on `main`. Otherwise stop and tell the human instead of creating a duplicate. These checks gate implementation, not reading or investigating code. If you cannot access GitHub, ask the human to confirm.
- If the issue has the `No AI` label, do not work on it. Stop and tell the human.
- Implement the approach agreed in the issue. New features need maintainer agreement and a KEP ([features.md](Documentation/contributor-guide/features.md)); refactors of stable code, new tools, and new platforms need maintainer agreement in the issue first.
- Each change must fix a problem the human has confirmed and can explain. Do not open speculative pull requests from automated code scanning.
- Keep one concern per pull request, even a small one: refactors (including test refactors), API changes, dependency bumps, and unrelated flake fixes go in separate pull requests. Do not fix unrelated lint warnings or reformat untouched code.

## Security

- Never open a public issue or pull request for a suspected vulnerability. Stop and tell the human; reports go through the private [Security Vulnerability Reporting Form](https://github.com/etcd-io/etcd/security/advisories/new). See [security/README.md](security/README.md) and the [threat model](THREAT_MODEL.md).
- The certificates and `*.key.insecure` files in `tests/fixtures/` are test fixtures, not leaked secrets.

## Repository layout

- etcd is a multi-module Go workspace ([go.work](go.work)); see [modules.md](Documentation/contributor-guide/modules.md). Raft and bbolt are separate repositories ([etcd-io/raft](https://github.com/etcd-io/raft), [etcd-io/bbolt](https://github.com/etcd-io/bbolt)); changes to their internals belong there.
- Modules with a `.gomodguard.yaml` (every library module except `cache/`) list imports they must not use, such as `go.etcd.io/etcd/tests/v3`, or `server` and `pkg` from client modules. `make verify-gomodguard` enforces this.
- Unit tests live next to the code they test. Tests that need a running cluster go under `tests/`.
- `client/v3/example*_test.go` and `client/v3/concurrency/example*_test.go` are symlinks to files under `tests/integration/clientv3/`. Edit the targets; never replace a symlink with a regular file.

## Environment

- The Go version is pinned in [.go-version](.go-version) and the `toolchain` directive of each `go.mod`; with the default `GOTOOLCHAIN=auto`, Go downloads it. Do not change Go versions unless that is the task.
- Only `linux-amd64` is officially supported for development. Do not add platform-specific workarounds.
- Besides Go, the build and checks use `make`, `jq`, `xz`, `yamllint`, `wget`, and `unzip`. Scripts download `shellcheck` and `protoc` (v3.20.3) when missing.

## Build and test

Run commands from the repository root; the workspace lets `go test` target packages in any module.

```bash
make build              # builds bin/etcd, bin/etcdctl, bin/etcdutl
make test-unit          # unit tests for all modules: -short, -race on amd64/arm64, ETCD_VERIFY=all
make test-integration   # integration tests (slow)
make test-e2e           # e2e tests, runs make build first (slow)
make test-e2e-release   # e2e including mixed-version cases; downloads bin/etcd-last-release
```

While iterating, run the tests that cover the behavior you changed, including existing tests you did not edit:

```bash
# unit test; ETCD_VERIFY=all enables the invariant checks that make test-unit runs
ETCD_VERIFY=all go test ./server/storage/mvcc -run 'TestKVPutMultipleTimes$' -count=1

# common framework test against an in-process cluster
go test -tags=integration ./tests/common -run 'TestKVPut$' -count=1

# common framework or e2e test against the binaries in bin/
go test -tags=e2e ./tests/common -run 'TestKVPut$' -count=1
go test ./tests/e2e -run 'TestForceNewCluster$' -count=1
```

Set `EXPECT_DEBUG=true` to print e2e process output, or `CLUSTER_DEBUG=1` for debug logs from integration cluster members.

### Tests that report ok without running

- Failpoint tests skip unless the code is built with gofail. A `SKIP` that mentions `make gofail-enable` means the test did not run, even though `go test` reports `ok`. Run `make gofail-enable` (then `make build` for e2e), run the tests, and always finish with `make gofail-disable`. Before committing, `git status` must show no `*.fail.go` files and no gofail changes to `go.mod` or `go.sum`.
- Mixed-version, upgrade, and downgrade cases skip unless `bin/etcd-last-release` exists. Use `make test-e2e-release` for changes that affect compatibility.
- e2e tests, and `tests/common` with `-tags=e2e`, use the binaries in `bin/` and never rebuild them. Run `make build` before them whenever the code changed since the last build.
- CI also runs `tests/integration` and `tests/e2e` through the gRPC proxy (`make test-grpcproxy-integration`, `make test-grpcproxy-e2e`). Tests that cannot work through the proxy belong in files tagged `//go:build !cluster_proxy`.

### Writing tests

- Every code change comes with a unit test; new features also need integration or e2e tests. For a bug fix, add a test that fails without the fix and say so in the pull request. For a flake fix, show `stress` or `-count=100` results before and after ([CONTRIBUTING.md](CONTRIBUTING.md#check-for-flaky-tests)).
- Extend existing test files and table-driven cases (`t.Run` with named cases) instead of adding new files or helpers.
- Prefer `tests/common/` for cluster-level tests ([etcd-io/etcd#13637](https://github.com/etcd-io/etcd/issues/13637)). When migrating a test there, delete the old e2e and integration copies in the same pull request.
- A new `tests/common` client operation needs changes in `tests/framework/interfaces/interface.go`, `tests/framework/e2e/etcdctl.go`, and `tests/framework/integration/integration.go`. Check with `go vet -tags=e2e ./tests/common` and `go vet -tags=integration ./tests/common`; the linter skips build-tagged files.
- In new e2e tests, use `e2e.NewEtcdProcessCluster` with the typed `epc.Etcdctl()` client. Do not copy the older `testCtl`/`ctlCtx` and `ctlV3*` helpers that match output with `SpawnWithExpect`.
- Call `integration.BeforeTest(t)`, `e2e.BeforeTest(t)`, or `testRunner.BeforeTest(t)` exactly once per test, and always `defer clus.Terminate(t)` or `epc.Close()`; the frameworks do not register cleanup.
- Do not call `t.Parallel()` in integration, e2e, common, or robustness tests. The frameworks change the working directory and use global state and fixed ports.
- Use `zaptest.NewLogger(t)` for loggers and `integration.NewClient(t, ...)` for integration clients. Wait with `require.Eventually` or the log-expect helpers instead of a fixed `time.Sleep`, and avoid tight timing assertions that flake on slow CI.
- Use `require`/`assert` from testify, with the `f` variants when passing a message (`require.NoErrorf`); `testifylint` enforces this.
- For failpoints, use `integration.BeforeTest(t, integration.WithFailpoint(...))` or `integration.SkipIfNoGoFail(t)`, and `e2e.WithGoFailEnabled(true)` in e2e. Setting `GOFAIL_FAILPOINTS` alone silently does nothing on normal binaries.

## Static checks

```bash
make verify             # all static checks; must pass before asking for review
make verify-lint        # any verify-* target in the Makefile can run on its own
make fix                # fixes what it can: lint, go.mod tidiness, dependency versions, bill of materials, go.work, toolchain directives, YAML, shell whitespace
```

- `make fix` can touch many files across modules. Review its diff and keep only the changes your task needs.
- Lint with `make verify-lint` or `make fix-lint`, not bare `golangci-lint`: the config is [tools/.golangci.yaml](tools/.golangci.yaml) and it runs once per module.
- Markdown changes are linted with markdownlint using [tools/.markdownlint.jsonc](tools/.markdownlint.jsonc) (`make markdown-diff-lint`).

## Generated and managed files

Do not edit these by hand.

| Files | Regenerate with |
| --- | --- |
| `*.pb.go`, `gw/*.pb.gw.go`, `Documentation/dev-guide/apispec/swagger/*.json` | `./scripts/genproto.sh` after editing `.proto` files |
| `scripts/etcd_version_annotations.txt` | `./scripts/update_proto_annotations.sh`, after `genproto.sh` |
| `bill-of-materials.json` | `make fix-bom` |
| `go.work`, `go.work.sum` | `make update-go-workspace` (part of `make fix`) |
| Certificates and keys in `tests/fixtures/` | `./gencerts.sh`, run from `tests/fixtures/` (needs `cfssl` from `tools/mod`) |

Dependency changes follow [dependency_management.md](Documentation/contributor-guide/dependency_management.md). Bump only direct dependencies (indirect ones only for a reported vulnerability), keep versions identical across modules (`./scripts/update_dep.sh <module> [version]`, checked by `make verify-dep`), and do not add a new dependency without maintainer agreement.

## Code conventions

- Before adding a file, dependency, helper, or abstraction, look for an existing equivalent and extend it. Follow the module boundaries, naming, and directory structure already in place, and explain in the pull request why anything new is needed.
- Mirror neighboring code: follow the structure of sibling functions, keep receiver names consistent, put methods next to their type, do not shadow builtins such as `cap`, and do not add wrappers that only forward a call.
- Comments explain a non-obvious why, where the reader needs it. Do not add comments that restate the code or narrate the change.
- Follow [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) and [tools/.golangci.yaml](tools/.golangci.yaml). Rules that differ from defaults: every `.go` file starts with the license header in [.header](.header), with the current year on new files (`goheader`); `GRPC` and `WAL` stay upper-case in identifiers (`revive`); no variable shadowing (`govet`); compare errors with `errors.Is`/`errors.As` (`errorlint`).
- Sentinel error messages start with the component, for example `errors.New("etcdserver: ...")`. In files that import `go.etcd.io/etcd/server/v3/etcdserver/errors`, import the standard library package as `errorspkg "errors"`.
- Return errors instead of panicking in tools and test libraries, and never use an error to signal success. Do not remove existing panics in the apply or storage path; they are intentional.
- Log through the existing zap logger with structured, kebab-case keys (`zap.String("local-member-id", id.String())`) and lowercase messages. Choose levels according to [logging.md](Documentation/contributor-guide/logging.md).
- `// gofail:` comments (both `var` and `label` forms) are failpoints ([gofail](https://github.com/etcd-io/gofail)). Do not remove or rename them. A new failpoint only works if its package is listed in the `gofail-enable` and `gofail-disable` targets of [tests/robustness/Makefile](tests/robustness/Makefile).
- Protobuf code uses `google.golang.org/protobuf`, not gogo: marshal with `proto.Marshal` or `pbutil.MustMarshalMessage`, use generated enum names such as `mvccpb.Event_PUT`, and compare with `protocmp` or `proto.Equal`.

## Server invariants

- The apply path (`server/etcdserver/apply`, `server/etcdserver/txn`, mvcc writes, auth and lease apply) must be deterministic across members. Generate IDs, tokens, hashes, and timestamps in `server/etcdserver/v3_server.go` before proposing, and carry them in the request. In write transactions, validate in `checkTxn`/`checkRange` before writing.
- Lock backend transactions with `LockInsideApply()` on the apply path and `LockOutsideApply()` elsewhere, never `Lock()`. These rules are only checked with `ETCD_VERIFY=all`.
- Errors returned to `server/etcdserver/api/v3rpc` must be the unwrapped sentinels in `toGRPCErrorMap`. It matches by identity, so a `%w`-wrapped error becomes `codes.Unknown`. Never change the text of an existing `rpctypes` error; clients match on it.
- Auth, quota, and space checks live in wrapper types that embed the next layer (`authApplierV3`, `quotaApplierV3`, `applierV3Capped`, `applierV3Corrupt`, `authMaintenanceServer`, `quotaKVServer`, `quotaLeaseServer`). A new applier method or KV, Lease, or Maintenance RPC must be overridden in each relevant wrapper, or the checks are silently skipped.
- Members panic on raft request types they do not know (`server/etcdserver/apply/uber_applier.go`). Only propose a new `InternalRaftRequest` type once the cluster version supports it, and never reuse a proto field number.

## Adding API surface

- Proto fields, messages, and enum values under `api/`: annotate them with `etcd_version_field`, `etcd_version_msg`, `etcd_version_enum`, or `etcd_version_enum_value` set to the next minor version (major.minor of `Version` in [api/version/version.go](api/version/version.go)). Then run `./scripts/genproto.sh` and `./scripts/update_proto_annotations.sh`. `make verify-proto-annotations` does not catch a missing annotation. Response header fields are also copied by hand in `server/etcdserver/api/v3rpc/watch.go` and `client/v3/leasing/util.go`.
- New gRPC error: add `ErrGRPCX`, its `errStringToError` entry, and the client `ErrX` in `api/v3rpc/rpctypes/error.go`, plus a `toGRPCErrorMap` entry in `server/etcdserver/api/v3rpc/util.go`. Prefer returning an existing error; every new exported error has to be maintained.
- New RPC: besides the proto and server handler, update the gRPC proxy (`server/proxy/grpcproxy`), the retry policy in `client/v3/retry.go`, every in-repo implementation of the affected client interface (leasing, namespace, ordering, mocks and fakes), the `tests/framework` clients, and `isRPCSupportedForLearner` if learners may serve it.
- New server flag: add an `embed.Config` field whose `json` tag equals the flag name, set its default in `NewConfig` and pass `cfg.Field` as the default in `AddFlags`, add it to `server/etcdmain/help.go` (`TestFlagsPresentInHelp` checks this), and copy it into `config.ServerConfig` in `server/embed/etcd.go`. Prefer extending an existing flag. Never add `--experimental-*` flags (removed in v3.7); gate new behavior with a feature gate.
- New feature gate: add it to the constants and to `DefaultEtcdServerFeatureGates` in `server/features/etcd_features.go`, with the comment template there (owner, KEP or issue, `alpha: v3.X`). Features usually start as Alpha ([features.md](Documentation/contributor-guide/features.md)).
- New bucket or meta key: add a schema change for the next version to `schemaChanges` in `server/storage/schema/schema.go` and update `AllBuckets`; add per-member values to `DefaultIgnores`.
- etcdctl: commands use `Run:` (not `RunE:`) and report errors with `cobrautl.ExitWithError`. New output fields must be added to the `make*Table` helpers in `printer.go`, to `printer_fields.go`, and to the `Hex*` types in `printer_json.go`.
- Metrics: put new metrics that may still change under the `etcd_debugging` namespace. Do not rename or remove an `etcd_` metric without a deprecation period.
- Exported identifiers in `api/`, `client/`, and `pkg/`: never remove or rename them outright. Add the replacement, keep the old one with a `// Deprecated: use X instead.` comment, and add a CHANGELOG Deprecations entry.

## Legacy code

- Do not build on `server/etcdserver/api/v2store`, the `storev2.go` helpers, or `ApplyV2storeOnly`; the v2 API is gone. `v2stats` is still in use.
- Comments such as "remove in v3.7" or `TODO(v3.4)` are not tasks. Do not act on them without an issue.

## Before handing off

- Review the final diff for unrelated changes, generated or gofail artifacts, and unnecessary additions.
- Tell the human what changed, which checks ran and their results, and which checks could not run and why.
- Do not comment, file issues, or take other GitHub actions unless the human asks; hand findings to the human instead.

## Commits and pull requests

- Commit subjects roughly follow `<package>: <what changed>`, for example `etcdserver: add grpc interceptor to log info on incoming requests`.
- Every commit needs a Developer Certificate of Origin sign-off (`git commit -s`); the `DCO` check fails pull requests without it.
- Before review, squash fixups into one commit, or a few self-contained ones such as a failing test followed by the fix, rebased on the target branch with no merge commits.
- Follow the [Kubernetes AI guidance](https://github.com/kubernetes/community/blob/master/contributors/guide/pull-requests.md#ai-guidance):
  - Do not add `Co-authored-by`, `Assisted-by`, or similar trailers that credit an AI tool. Some tools add one by default; remove it.
  - Do not write commit messages or replies to reviewers on the human's behalf. Leave them to the human.
  - The pull request description must disclose AI use, for example "This PR was written in part with the assistance of generative AI."
  - The human author must understand and be able to explain every change.
- Do not put `fixes #123`-style keywords or `@mentions` in commit messages. Link the issue in the pull request description.
- Keep pull requests small; split anything over about 500 lines.
- CHANGELOG: add entries for features, user-visible changes, Go version bumps, and CVE-driven dependency bumps, not routine dependency bumps. Put each entry under the matching component heading, linking the pull request. Features go in the newest `CHANGELOG/CHANGELOG-3.*.md`; bug fixes go in the changelog of each release branch they are backported to, linking the backport pull request, not in the unreleased version's changelog.
- Backports: fixes land on `main` first and are backported with a `/cherry-pick release-3.x` comment ([cherry-pick.md](Documentation/contributor-guide/cherry-pick.md)). Use `./scripts/cherrypick.sh` only if the bot fails, title backports `[release-3.x] ...`, and do not backport features, dependency bumps without a CVE, or anything to end-of-life branches such as `release-3.4`.
- Pull requests from first-time contributors need `/ok-to-test` from an etcd-io member before CI runs. If an unrelated test fails because it is flaky, tell the human so they can file an issue to deflake it and ask a maintainer to rerun the tests.
