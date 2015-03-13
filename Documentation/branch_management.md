## Branch Managemnt

### Guide

- New development occurs on the master branch
- Master branch should always have a green build!
- Backwards-compatible bug fixes should target the master branch and ported to stable
- Once the master branch is ready for release, it will be tagged and become the new stable branch.

The etcd team adopts a rolling release model and support one stable version of etcd going forward.

### Master branch

The master branch is our development branch. It is where all the new features go into first.

If you want to try new features, pull the master branch and play on it. But the branch is not really stable because new features may introduce bugs.

Before the release of the next stable version, feature PRs will be frozen. We will focus on the testing, bug-fix and documentation for one to two weeks.

### Stable branches

All branches with prefix 'release-' are stable branches.

After a Minor release (http://semver.org/), we will have a new stable branch for that release. We will keep fixing the backwards-compatible bugs for the latest stable release, but not the olders. The bug fixes Patch release will be once every two weeks, given any patches.
