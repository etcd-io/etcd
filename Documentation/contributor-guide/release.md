# Release

The guide talks about how to release a new version of etcd.

The procedure includes some manual steps for sanity checking, but it can probably be further scripted. Please keep this document up-to-date if making changes to the release process.

## Release management

The following pool of release candidates manage the release each etcd major/minor version as well as manage patches
to each stable release branch. They are responsible for communicating the timelines and status of each release and
for ensuring the stability of the release branch.

- Benjamin Wang [@ahrtr](https://github.com/ahrtr)
- James Blair [@jmhbnz](https://github.com/jmhbnz)
- Marek Siarkowicz [@serathius](https://github.com/serathius)
- Sahdev Zala [@spzala](https://github.com/spzala)
- Wenjia Zhang [@wenjiaswe](https://github.com/wenjiaswe)

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

## Patch release criteria

The etcd project aims to release a new patch version if any of the following conditions are met:

- Fixed one or more major CVEs (>=7.5).
- Fixed one or more critical bugs.
- Fixed three or more major bugs.
- Fixed five or more minor bugs.

## Release guide

### Prerequisites

There are some prerequisites, which should be done before the release process. These are one time operations,
which don't need to be executed before releasing each version.
1. Generate a GPG key and add it to your github account. Refer to the links on [settings](https://github.com/settings/keys).
2. Ensure you have a Linux machine, on which the git, golang and docker have been installed.
    - Ensure the golang version matches the version defined in `.go-version` file.
    - Ensure non-privileged users can run docker commands, refer to the [Linux postinstall](https://docs.docker.com/engine/install/linux-postinstall/).
    - Ensure there is at least 500MB free space on your Linux machine.
3. Install gsutil, refer to [gsutil_install](https://cloud.google.com/storage/docs/gsutil_install).
4. Authenticate the image registry, refer to [Authentication methods](https://cloud.google.com/container-registry/docs/advanced-authentication).
   - `gcloud auth login`
   - `gcloud auth configure-docker`

### Release steps

1. Raise an issue to public the release plan, e.g. [issues/17350](https://github.com/etcd-io/etcd/issues/17350).
2. Verify you can pass the authentication to the image registries,
   - `docker login gcr.io`
   - `docker login quay.io`
3. Clone the etcd repository and checkout the target branch,
   - `git clone git@github.com:etcd-io/etcd.git`
   - `git checkout release-3.X`
4. Run the release script under the repository's root directory,
   - `DRY_RUN=false ./scripts/release.sh ${VERSION}`

   It generates all release binaries under directory ./release and images. Binaries are pushed to the google cloud bucket
   under project `etcd-development`, and images are pushed to `quay.io` and `gcr.io`.
5. Publish release page in Github
   - Set release title as the version name
   - Follow the format of previous release pages
   - Attach the generated binaries and signature file
   - Select whether it's a pre-release
   - Publish the release
6. Announce to the etcd-dev googlegroup

   Follow the format of previous release emails sent to etcd-dev@googlegroups.com, see an example below,
```
Hello,

etcd v3.4.30 is now public!

https://github.com/etcd-io/etcd/releases/tag/v3.4.30

Thanks to everyone who contributed to the release!

etcd team
```
7. Update the changelog to reflect the correct release date.
8. Paste the release link to the issue raised at step 1 and close the issue.
9. Crease a new stable branch through `git push origin release-${VERSION_MAJOR}.${VERSION_MINOR}` if this is a new major or minor stable release.
