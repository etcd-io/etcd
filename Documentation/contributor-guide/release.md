# Release

The guide talks about how to release a new version of etcd.

The procedure includes some manual steps for sanity checking, but it can probably be further scripted. Please keep this document up-to-date if making changes to the release process.

## Release management

etcd community members are assigned to manage the release each etcd major/minor version as well as manage patches
and to each stable release branch. The managers are responsible for communicating the timelines and status of each
release and for ensuring the stability of the release branch.

| Releases               | Manager                                                     |
|------------------------|-------------------------------------------------------------|
| 3.4 patch (post 3.4.0) | Benjamin Wang [@ahrtr](https://github.com/ahrtr)            |
| 3.5 patch (post 3.5.0) | Marek Siarkowicz [@serathius](https://github.com/serathius) |

All releases version numbers follow the format of [semantic versioning 2.0.0](http://semver.org/).

### Major, minor version release, or its pre-release

- Ensure the relevant milestone on GitHub is complete. All referenced issues should be closed, or moved elsewhere.
- Ensure the latest upgrade documentation is available.
- Bump [hardcoded MinClusterVerion in the repository](https://github.com/etcd-io/etcd/blob/v3.4.15/version/version.go#L29), if necessary.
- Add feature capability maps for the new version, if necessary.

### Patch version release

- To request a backport, devlopers submit cherrypick PRs targeting the release branch. The commits should not include merge commits. The commits should be restricted to bug fixes and security patches.
- The cherrypick PRs should target the appropriate release branch (`base:release-<major>-<minor>`). `hack/patch/cherrypick.sh` may be used to automatically generate cherrypick PRs.
- The release patch manager reviews the cherrypick PRs. Please discuss carefully what is backported to the patch release. Each patch release should be strictly better than it's predecessor.
- The release patch manager will cherry-pick these commits starting from the oldest one into stable branch.

## Write release note

- Write introduction for the new release. For example, what major bug we fix, what new features we introduce or what performance improvement we make.
- Put `[GH XXXX]` at the head of change line to reference Pull Request that introduces the change. Moreover, add a link on it to jump to the Pull Request.
- Find PRs with `release-note` label and explain them in `NEWS` file, as a straightforward summary of changes for end-users.

## Build and push the release artifacts

- Ensure `docker` is available.

Run release script in root directory:

```
DRY_RUN=false ./scripts/release.sh ${VERSION}
```

It generates all release binaries and images under directory ./release.
Binaries are pushed to gcr.io and images are pushed to quay.io and gcr.io.

## Publish release page in GitHub

- Set release title as the version name.
- Follow the format of previous release pages.
- Attach the generated binaries and signatures.
- Select whether it is a pre-release.
- Publish the release!

## Announce to the etcd-dev Googlegroup

- Follow the format of [previous release emails](https://groups.google.com/g/etcd-dev).
- Make sure to include a list of authors that contributed since the previous release - something like the following might be handy:

```
git log ...${PREV_VERSION} --pretty=format:"%an" | sort | uniq | tr '\n' ',' | sed -e 's#,#, #g' -e 's#, $##'
```

- Send email to etcd-dev@googlegroups.com

## Post release

- Create new stable branch through `git push origin ${VERSION_MAJOR}.${VERSION_MINOR}` if this is a major stable release. This assumes `origin` corresponds to "https://github.com/etcd-io/etcd".
- Bump [hardcoded Version in the repository](https://github.com/etcd-io/etcd/blob/v3.4.15/version/version.go#L30) to the version `${VERSION}+git`.
