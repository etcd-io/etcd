# Branch management

## Guide

* New development occurs on the [main branch][main].
* Main branch should always have a green build!
* Backwards-compatible bug fixes should target the main branch and subsequently be ported to stable branches.
* Once the main branch is ready for release, it will be tagged and become the new stable branch.

The etcd team has adopted a *rolling release model* and supports two stable versions of etcd.

### Main branch

The `main` branch is our development branch. All new features land here first.

To try new and experimental features, pull `main` and play with it. Note that `main` may not be stable because new features may introduce bugs.

Before the release of the next stable version, feature PRs will be frozen. A [release manager](../dev-internal/release/#release-management) will be assigned to major/minor version and will lead the etcd community in test, bug-fix and documentation of the release for one to two weeks.

### Stable branches

All branches with prefix `release-` are considered _stable_ branches.

After every minor release ([semver.org](https://semver.org/)), we will have a new stable branch for that release, managed by a [patch release manager](../dev-internal/release/#release-management). We will keep fixing the backwards-compatible bugs for the latest two stable releases. A _patch_ release to each supported release branch, incorporating any bug fixes, will be once every two weeks, given any patches.

[main]: https://github.com/etcd-io/etcd/tree/main

