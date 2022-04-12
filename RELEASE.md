# Release

The guide talks about how to release a new version of etcd.

The procedure includes some manual steps for sanity checking, but it can probably be further scripted. Please keep this document up-to-date if making changes to the release process.

## Release management

etcd community members are assigned to manage the release each etcd major/minor version as well as manage patches
and to each stable release branch. The managers are responsible for communicating the timelines and status of each
release and for ensuring the stability of the release branch.

| Releases | Manager |
| -------- | ------- |
| 3.1 patch (post 3.1.0) | Joe Betz [@jpbetz](https://github.com/jpbetz) |
| 3.2 patch (post 3.2.0) | Joe Betz [@jpbetz](https://github.com/jpbetz) |
| 3.3 patch (post 3.3.0) | Gyuho Lee [@gyuho](https://github.com/gyuho) |

## Prepare release

Set desired version as environment variable for following steps. Here is an example to release 2.3.0:

```
export VERSION=v2.3.0
export PREV_VERSION=v2.2.5
```

All releases version numbers follow the format of [semantic versioning 2.0.0](http://semver.org/).

### Major, minor version release, or its pre-release

- Ensure the relevant milestone on GitHub is complete. All referenced issues should be closed, or moved elsewhere.
- Remove this release from [roadmap](https://github.com/etcd-io/etcd/blob/master/ROADMAP.md), if necessary.
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

## Tag version

- Bump [hardcoded Version in the repository](https://github.com/etcd-io/etcd/blob/v3.4.15/version/version.go#L30) to the latest version `${VERSION}`.
- Ensure all tests on CI system are passed.
- Manually check etcd is buildable in Linux, Darwin and Windows.
- Manually check upgrade etcd cluster of previous minor version works well.
- Manually check new features work well.
- Add a signed tag through `git tag -s ${VERSION}`.
- Sanity check tag correctness through `git show tags/$VERSION`.
- Push the tag to GitHub through `git push origin tags/$VERSION`. This assumes `origin` corresponds to "https://github.com/etcd-io/etcd\".

## Build release binaries and images

- Ensure `docker` is available.

Run release script in root directory:

```
TAG=gcr.io/etcd-development/etcd ./scripts/release.sh ${VERSION}
```

It generates all release binaries and images under directory ./release.

## Sign binaries, images, and source code

etcd project key must be used to sign the generated binaries and images.`$SUBKEYID` is the key ID of etcd project Yubikey. Connect the key and run `gpg2 --card-status` to get the ID.

The following commands are used for public release sign:

```
cd release
for i in etcd-*{.zip,.tar.gz}; do gpg2 --default-key $SUBKEYID --armor --output ${i}.asc --detach-sign ${i}; done
for i in etcd-*{.zip,.tar.gz}; do gpg2 --verify ${i}.asc ${i}; done

# sign zipped source code files
wget https://github.com/etcd-io/etcd/archive/${VERSION}.zip
gpg2 --armor --default-key $SUBKEYID --output ${VERSION}.zip.asc --detach-sign ${VERSION}.zip
gpg2 --verify ${VERSION}.zip.asc ${VERSION}.zip

wget https://github.com/etcd-io/etcd/archive/${VERSION}.tar.gz
gpg2 --armor --default-key $SUBKEYID --output ${VERSION}.tar.gz.asc --detach-sign ${VERSION}.tar.gz
gpg2 --verify ${VERSION}.tar.gz.asc ${VERSION}.tar.gz
```

The public key for GPG signing can be found at [CoreOS Application Signing Key](https://coreos.com/security/app-signing-key)


## Publish release page in GitHub

- Set release title as the version name.
- Follow the format of previous release pages.
- Attach the generated binaries and signatures.
- Select whether it is a pre-release.
- Publish the release!

## Publish docker image in gcr.io

- Push docker image:

```
gcloud docker -- login -u _json_key -p "$(cat /etc/gcp-key-etcd.json)" https://gcr.io

for TARGET_ARCH in "-arm64" "-ppc64le" "-s390x" ""; do
  gcloud docker -- push gcr.io/etcd-development/etcd:${VERSION}${TARGET_ARCH}
done
```

- Add `latest` tag to the new image on [gcr.io](https://console.cloud.google.com/gcr/images/etcd-development/GLOBAL/etcd?project=etcd-development&authuser=1) if this is a stable release.

## Publish docker image in Quay.io

- Build docker images with quay.io:

```
for TARGET_ARCH in "amd64" "arm64" "ppc64le" "s390x"; do
  TAG=quay.io/coreos/etcd GOARCH=${TARGET_ARCH} \
    BINARYDIR=release/etcd-${VERSION}-linux-${TARGET_ARCH} \
    BUILDDIR=release \
    ./scripts/build-docker ${VERSION}
done
```

- Push docker image:

```
docker login quay.io

for TARGET_ARCH in "-arm64" "-ppc64le" "-s390x" ""; do
  docker push quay.io/coreos/etcd:${VERSION}${TARGET_ARCH}
done
```

- Add `latest` tag to the new image on [quay.io](https://quay.io/repository/coreos/etcd?tag=latest&tab=tags) if this is a stable release.

## Announce to the etcd-dev Googlegroup

- Follow the format of [previous release emails](https://groups.google.com/forum/#!forum/etcd-dev).
- Make sure to include a list of authors that contributed since the previous release - something like the following might be handy:

```
git log ...${PREV_VERSION} --pretty=format:"%an" | sort | uniq | tr '\n' ',' | sed -e 's#,#, #g' -e 's#, $##'
```

- Send email to etcd-dev@googlegroups.com

## Post release

- Create new stable branch through `git push origin ${VERSION_MAJOR}.${VERSION_MINOR}` if this is a major stable release. This assumes `origin` corresponds to "https://github.com/etcd-io/etcd\".
- Bump [hardcoded Version in the repository](https://github.com/etcd-io/etcd/blob/v3.4.15/version/version.go#L30) to the version `${VERSION}+git`.