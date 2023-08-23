# Dependency management

## Table of Contents

- **[Main branch](#main-branch)**
  - [Dependencies used in workflows](#dependencies-used-in-workflows)
  - [Bumping order](#bumping-order)
  - [Steps to bump a dependency](#steps-to-bump-a-dependency)
  - [Indirect dependencies](#indirect-dependencies)
  - [Known incompatible dependency updates](#known-incompatible-dependency-updates)
    - [arduino/setup-protoc](#arduinosetup-protoc)
  - [Rotation worksheet](#rotation-worksheet)
- **[Stable branches](#stable-branches)**
- **[Golang versions](#golang-versions)**

## Main branch

The dependabot is enabled & [configured](https://github.com/etcd-io/etcd/blob/main/.github/dependabot.yml) to
manage dependencies for etcd `main` branch. But dependabot doesn't work well for multi-module repository like `etcd`,
see [dependabot-core/issues/6678](https://github.com/dependabot/dependabot-core/issues/6678).
Usually human intervention is required each time when dependabot automatically opens some PRs to bump dependencies.
Please see guidance below.

### Dependencies used in workflows

The PRs which automatically bump dependencies (see examples below) used in workflows are fine, and can be approved & merged directly as long as all checks are successful.

- [build(deps): bump github/codeql-action from 2.2.11 to 2.2.12](https://github.com/etcd-io/etcd/pull/15736)
- [build(deps): bump actions/checkout from 3.5.0 to 3.5.2](https://github.com/etcd-io/etcd/pull/15735)
- [build(deps): bump ossf/scorecard-action from 2.1.2 to 2.1.3](https://github.com/etcd-io/etcd/pull/15607)

### Bumping order

When multiple etcd modules depend on the same package, please bump the package version for all the modules in the correct order. The rule is simple:
if module A depends on module B, then bump the dependency for module B before module A. If the two modules do not depend on each other, then
it doesn't matter to bump which module first. For example, multiple modules depend on `github.com/spf13/cobra`, we need to bump the dependency
in the following order,

- go.etcd.io/etcd/pkg/v3
- go.etcd.io/etcd/server/v3
- go.etcd.io/etcd/etcdctl/v3
- go.etcd.io/etcd/etcdutl/v3
- go.etcd.io/etcd/tests/v3
- go.etcd.io/etcd/v3
- go.etcd.io/etcd/tools/v3
For more details about etcd Golang modules, please check <https://etcd.io/docs/next/dev-internal/modules>

Note the module `go.etcd.io/etcd/tools/v3` doesn't depend on any other modules, nor by any other modules, so it doesn't matter when to bump dependencies for it.

### Steps to bump a dependency

Use the `github.com/spf13/cobra` as an example, follow steps below to bump it from 1.6.1 to 1.7.0 for module `go.etcd.io/etcd/etcdctl/v3`,

```bash
cd ${ETCD_ROOT_DIR}/etcdctl
go get github.com/spf13/cobra@v1.7.0
go mod tidy
cd ..
./scripts/fix.sh
```

Execute the same steps for all other modules. When you finish bumping the dependency for all modules, then commit the change,

```bash
git add .
git commit --signoff -m "dependency: bump github.com/spf13/cobra from 1.6.1 to 1.7.0"
```

Please close the related PRs which were automatically opened by dependabot. 

When you bump multiple dependencies in one PR, it's recommended to create a separate commit for each dependency. But it isn't a must; for example,
you can get all dependencies bumping for the module `go.etcd.io/etcd/tools/v3` included in one commit.

### Indirect dependencies

Usually we don't bump a dependency if all modules just indirectly depend on it, such as `github.com/go-logr/logr`.

If an indirect dependency (e.g. `D1`) causes any CVE or bugs which affect etcd, usually the module (e.g. `M1`, not part of etcd, but used by etcd)
which depends on it should bump the dependency (`D1`), and then etcd just needs to bump `M1`. However, if the module (`M1`) somehow doesn't
bump the problematic dependency, then etcd can still bump it (`D1`) directly following the same steps above. But as a long-term solution, etcd should
try to remove the dependency on such module (`M1`) that lack maintenance.

For mixed cases, in which some modules directly while others indirectly depend on a dependency, we have multiple options,

- Bump the dependency for all modules, no matter it's direct or indirect dependency.
- Bump the dependency only for modules which directly depend on it.

We should try to follow the first way, and temporarily fall back to the second one if we run into any issue on the first way. Eventually we
should fix the issue and ensure all modules depend on the same version of the dependency.

### Known incompatible dependency updates

#### arduino/setup-protoc

Please refer to [build(deps): bump arduino/setup-protoc from 1.3.0 to 2.0.0](https://github.com/etcd-io/etcd/pull/16016)

### Rotation worksheet

The dependabot scheduling interval is weekly; it means dependabot will automatically raise a bunch of PRs per week.
Usually human intervention is required each time. We have a [rotation worksheet](https://docs.google.com/spreadsheets/d/1DDWzbcOx1p32MhyelaPZ_SfYtAD6xRsrtGRZ9QXPOyQ/edit#gid=0),
and everyone is welcome to participate; you just need to register your name in the worksheet.

## Stable branches

Usually we don't proactively bump dependencies for stable releases unless there are any CVEs or bugs that affect etcd.

If we have to do it, then follow the same guidance above. Note that there is no `./scripts/fix.sh` in release-3.4, so no need to
execute it for 3.4.

## Golang versions

The etcd project aims to maintain a development branch that is on the latest [Go version](https://go.dev/dl), ideally this will align with the Go version in use for Kubernetes project development. For an example on how to update etcd to a new minor release of Go refer to issue <https://github.com/etcd-io/etcd/issues/16393> and the linked pull requests.

Suggested steps for performing a minor version upgrade for the etcd development branch:

1. Carefully review new Go version release notes and potentially related blog for any deprecations, performance impacts or other considerations.
2. Create a github issue to signal intent to upgrade and invite discussion, example <https://github.com/etcd-io/etcd/issues/16393>.
3. Complete the upgrade locally in your development environment.
4. Run performance benchmarks locally to compare before and after.
5. Raise a pull request for the changes, example <https://github.com/etcd-io/etcd/pull/16394>.

Stable etcd release branches will be maintained to stay on the latest patch release of a supported Go version, however upgrading minor versions will be avoided unless the minor version in use is now out of support. Refer to the [Go release policy](https://go.dev/doc/devel/release).

For an example of how to update etcd to a new patch release of Go refer to issue <https://github.com/etcd-io/etcd/issues/16343> and the linked pull requests.

References:

- <https://github.com/kubernetes/sig-release/blob/master/release-engineering/handbooks/go.md>
