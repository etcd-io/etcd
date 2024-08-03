# Release

The guide talks about how to release a new version of etcd.

The procedure includes some manual steps for sanity checking, but it can probably be further scripted. Please keep this document up-to-date if making changes to the release process.

## Release management

The following pool of release candidates manages the release of each etcd major/minor version as well as manages patches
to each stable release branch. They are responsible for communicating the timelines and status of each release and
for ensuring the stability of the release branch.

- Benjamin Wang [@ahrtr](https://github.com/ahrtr)
- James Blair [@jmhbnz](https://github.com/jmhbnz)
- Marek Siarkowicz [@serathius](https://github.com/serathius)
- Sahdev Zala [@spzala](https://github.com/spzala)
- Wenjia Zhang [@wenjiaswe](https://github.com/wenjiaswe)

All release version numbers follow the format of [semantic versioning 2.0.0](http://semver.org/).

### Major, minor version release, or its pre-release

- Ensure the relevant [milestone](https://github.com/etcd-io/etcd/milestones) on GitHub is complete. All referenced issues should be closed or moved elsewhere.
- Ensure the latest [upgrade documentation](https://etcd.io/docs/next/upgrades) is available.
- Bump [hardcoded MinClusterVerion in the repository](https://github.com/etcd-io/etcd/blob/v3.4.15/version/version.go#L29), if necessary.
- Add feature capability maps for the new version, if necessary.

### Patch version release

- To request a backport, developers submit cherry-pick PRs targeting the release branch. The commits should not include merge commits. The commits should be restricted to bug fixes and security patches.
- The cherrypick PRs should target the appropriate release branch (`base:release-<major>-<minor>`). The k8s infra cherry pick robot `/cherrypick <branch>` PR chatops command may be used to automatically generate cherrypick PRs.
- The release patch manager reviews the cherrypick PRs. Please discuss carefully what is backported to the patch release. Each patch release should be strictly better than its predecessor.
- The release patch manager will cherry-pick these commits starting from the oldest one into stable branch.

## Write a release note

- Write an introduction for the new release. For example, what major bug we fix, what new features we introduce, or what performance improvement we make.
- Put `[GH XXXX]` at the head of the change line to reference the Pull Request that introduces the change. Moreover, add a link on it to jump to the Pull Request.
- Find PRs with the `release-note` label and explain them in the `NEWS` file, as a straightforward summary of changes for end-users.

## Patch release criteria

The etcd project aims to release a new patch version if any of the following conditions are met:

- Fixed one or more major CVEs (>=7.5).
- Fixed one or more critical bugs.
- Fixed three or more major bugs.
- Fixed five or more minor bugs.

## Release guide

### Prerequisites

There are some prerequisites, which should be done before the release process. These are one-time operations,
which don't need to be executed before releasing each version.
1. Generate a GPG key and add it to your GitHub account. Refer to the links on [settings](https://github.com/settings/keys).
2. Ensure you have a Linux machine, on which the git, Golang, and docker have been installed.
    - Ensure the Golang version matches the version defined in `.go-version` file.
    - Ensure non-privileged users can run docker commands, refer to the [Linux postinstall](https://docs.docker.com/engine/install/linux-postinstall/).
    - Ensure there is at least 5GB of free space on your Linux machine.
3. Install gsutil, refer to [gsutil_install](https://cloud.google.com/storage/docs/gsutil_install). When asked about cloud project to use, pick `etcd-development`.
4. Authenticate the image registry, refer to [Authentication methods](https://cloud.google.com/container-registry/docs/advanced-authentication).
   - `gcloud auth login`
   - `gcloud auth configure-docker`

### Release steps

1. Raise an issue to publish the release plan, e.g. [issues/17350](https://github.com/etcd-io/etcd/issues/17350).
2. Raise a `kubernetes/org` pull request to temporarily elevate permissions for the GitHub release team.
3. Once permissions are elevated, temporarily relax [branch protections](https://github.com/etcd-io/etcd/settings/branches) to allow pushing changes directly to `release-*` branches in GitHub.
4. Verify you can pass the authentication to the image registries,
   - `docker login gcr.io`
   - `docker login quay.io`
5. Clone the etcd repository and checkout the target branch,
   - `git clone git@github.com:etcd-io/etcd.git`
   - `git checkout release-3.X`
6. Run the release script under the repository's root directory, replacing `${VERSION}` with a value without the `v` prefix, i.e. `3.5.13`.
   - `DRY_RUN=false ./scripts/release.sh ${VERSION}`

   It generates all release binaries under the directory `./release` and images. Binaries are pushed to the Google Cloud bucket
   under project `etcd-development`, and images are pushed to `quay.io` and `gcr.io`.
7. Publish the release page on GitHub
   - Set the release title as the version name
   - Choose the correct release tag (generated from step #4)
   - Follow the format of previous release pages
   - Attach the generated binaries and signature file
   - Select whether it's a pre-release
   - Publish the release
8. Announce to the etcd-dev googlegroup

   Follow the format of previous release emails sent to etcd-dev@googlegroups.com, see an example below. After sending out the email, ask one of the mailing list maintainers to approve the email from the pending list.

```text
Hello,

etcd v3.4.30 is now public!

https://github.com/etcd-io/etcd/releases/tag/v3.4.30

Thanks to everyone who contributed to the release!

etcd team
```

9. Update the changelog to reflect the correct release date.
10. Paste the release link to the issue raised in Step 1 and close the issue.
11. Restore standard branch protection settings and raise a follow-up `kubernetes/org` pull request to return to least privilege permissions.
12. Crease a new stable branch through `git push origin release-${VERSION_MAJOR}.${VERSION_MINOR}` if this is a new major or minor stable release.

#### Release known issues

1. Timeouts pushing binaries - If binaries fail to fully upload to Google Cloud storage, the script must be re-run using the same command. Any artifacts that are already pushed will be overwritten to ensure they are correct. The storage bucket does not use object versioning so incorrect files cannot remain.

2. Timeouts pushing images - It is rare, although possible for connection timeouts to occur when publishing etcd release images to `quay.io` or `gcr.io`. If this occurs, it is known to be safe to rerun the release script command appending the `--no-upload` flag, and image uploads will gracefully resume.

3. GPG vs SSH signing - The release scripts assume that git tags will be signed with a GPG key. Since 2022 GitHub has supported [signing commits and tags using ssh](https://github.blog/changelog/2022-08-23-ssh-commit-verification-now-supported). Until further release script updates are completed you will need to disable this feature in your `~/.gitconfig` and revert to signing via GPG to perform etcd releases.
