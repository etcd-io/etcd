#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source ./scripts/test_lib.sh

DRY_RUN=${DRY_RUN:-true}

help() {
  echo "$(basename "$0") [version]"
  echo "Release etcd using the same approach as the etcd-release-runbook (https://goo.gl/Gxwysq)"
  echo ""
  echo "WARNING: This does not perform the 'Add API capabilities', 'Performance testing' "
  echo "         or 'Documentation' steps. These steps must be performed manually BEFORE running this tool."
  echo ""
  echo "WARNING: This script does not send announcement emails. This step must be performed manually AFTER running this tool."
  echo ""
  echo "  args:"
  echo "    version: version of etcd to release, e.g. '3.2.18'"
  echo "  flags:"
  echo "    --in-place: build binaries using current branch."
  echo "    --no-docker-push: skip docker image pushes."
  echo "    --no-gh-release: skip creating the GitHub release using gh."
  echo "    --no-upload: skip gs://etcd binary artifact uploads."
  echo ""
}

main() {
  VERSION=$1
  if [[ ! "${VERSION}" =~ [0-9]+.[0-9]+.[0-9]+ ]]; then
    echo "Expected 'version' param of the form '<major-version>.<minor-version>.<patch-version>' but got '${VERSION}'"
    exit 1
  fi
  RELEASE_VERSION="v${VERSION}"
  MINOR_VERSION=$(echo "${VERSION}" | cut -d. -f 1-2)

  if [ "${IN_PLACE}" == 1 ]; then
      # Trigger release in current branch
      REPOSITORY=$(pwd)
      BRANCH=$(git rev-parse --abbrev-ref HEAD)
  else
      REPOSITORY=${REPOSITORY:-"git@github.com:etcd-io/etcd.git"}
      BRANCH=${BRANCH:-"release-${MINOR_VERSION}"}
  fi

  log_warning "DRY_RUN=${DRY_RUN}"
  log_callout "RELEASE_VERSION=${RELEASE_VERSION}"
  log_callout "MINOR_VERSION=${MINOR_VERSION}"
  log_callout "BRANCH=${BRANCH}"
  log_callout ""

  if ! command -v docker >/dev/null; then
    log_error "cannot find docker"
    exit 1
  fi

  # Expected umask for etcd release artifacts
  umask 022

  # Set up release directory.
  local reldir="/tmp/etcd-release-${VERSION}"
  log_callout "Preparing temporary directory: ${reldir}"
  if [ "${IN_PLACE}" == 0 ]; then
    if [ ! -d "${reldir}/etcd" ]; then
      mkdir -p "${reldir}"
      cd "${reldir}"
      git clone "${REPOSITORY}" --branch "${BRANCH}" --depth 1
    fi
    cd "${reldir}/etcd" || exit 2
    git checkout "${BRANCH}" || exit 2
    git pull origin
  fi

  # If a release version tag already exists, use it.
  log_callout "Checking tag: ${RELEASE_VERSION}"
  local remote_tag_exists
  remote_tag_exists=$(git ls-remote origin "refs/tags/${RELEASE_VERSION}" | grep -c "${RELEASE_VERSION}" || true)
  if [ "${remote_tag_exists}" -gt 0 ]; then
    log_callout "Release version tag exists on remote. Checking out refs/tags/${RELEASE_VERSION}"
    git checkout -q "tags/${RELEASE_VERSION}"
  fi

  # Check go version.
  log_callout "Checking go version"
  local go_version current_go_version
  go_version="go$(cat .go-version)"
  current_go_version=$(go version | awk '{ print $3 }')
  if [[ "${current_go_version}" != "${go_version}" ]]; then
    log_error "Current go version is ${current_go_version}, but etcd ${RELEASE_VERSION} requires ${go_version} (see .go-version)."
    exit 1
  fi

  if [ "${NO_GH_RELEASE}" == 1 ]; then
    echo "Skipping gh verification, --no-gh-release is set"
  else
    # Check that gh is installed and logged in.
    echo "Check gh installation"
    if ! command -v gh >/dev/null; then
      log_error "Cannot find gh. Please follow the installation instructions at https://github.com/cli/cli#installation"
      exit 1
    fi
    if ! gh auth status &>/dev/null; then
      log_error "GitHub authentication failed for gh. Please run gh auth login."
      exit 1
    fi
  fi

  # If the release tag does not already exist remotely, create it.
  log_callout "Create tag if not present"
  if [ "${remote_tag_exists}" -eq 0 ]; then
    # Bump version/version.go to release version.
    local source_version
    source_version=$(grep -E "\s+Version\s*=" version/version.go | sed -e "s/.*\"\(.*\)\".*/\1/g")
    if [[ "${source_version}" != "${VERSION}" ]]; then
      source_minor_version=$(echo "${source_version}" | cut -d. -f 1-2)
      if [[ "${source_minor_version}" != "${MINOR_VERSION}" ]]; then
        log_error "Wrong etcd minor version in version/version.go. Expected ${MINOR_VERSION} but got ${source_minor_version}. Aborting."
        exit 1
      fi
      echo "Updating version from ${source_version} to ${VERSION} in version/version.go"
      sed -i "s/${source_version}/${VERSION}/g" version/version.go
    fi

    log_callout "Building etcd and checking --version output"
    ./build
    local etcd_version
    etcd_version=$(bin/etcd --version | grep "etcd Version" | awk '{ print $3 }')
    if [[ "${etcd_version}" != "${VERSION}" ]]; then
      log_error "Wrong etcd version in version/version.go. Expected ${etcd_version} but got ${VERSION}. Aborting."
      exit 1
    fi

    if [[ -n $(git status -s) ]]; then
      log_callout "Committing version/version.go update."
      git add version/version.go
      git diff --staged | cat
      git commit --signoff -m "version: bump up to ${VERSION}"
      git diff --staged | cat
    fi

    # Push the version change if it's not already been pushed.
    if [ "$DRY_RUN" != "true" ] && [ "$(git rev-list --count "origin/${BRANCH}..${BRANCH}")" -gt 0 ]; then
      read -p "Push version bump up to ${VERSION} to ${REPOSITORY} [y/N]? " -r confirm
      [[ "${confirm,,}" == "y" ]] || exit 1
      git push
    fi

    # Tag release.
    if git tag --list | grep --quiet "^${RELEASE_VERSION}$"; then
      log_callout "Skipping tag step. git tag ${RELEASE_VERSION} already exists."
    else
      log_callout "Tagging release..."
      gitemail=$(git config --get user.email)
      KEYID=$(gpg --list-keys --with-colons "${gitemail}" | awk -F: '/^pub:/ { print $5 }')
      if [[ -z "${KEYID}" ]]; then
        log_error "Failed to load gpg key. Is gpg set up correctly for etcd releases?"
        exit 1
      fi
      git tag --local-user "${KEYID}" --sign "${RELEASE_VERSION}" --message "${RELEASE_VERSION}"

      if [ "$DRY_RUN" != "true" ]; then
        read -p "Push tag ${RELEASE_VERSION} to ${REPOSITORY} [y/N]? " -r confirm
        [[ "${confirm,,}" == "y" ]] || exit 1
        git push origin "${RELEASE_VERSION}"
      fi
    fi

    # Verify the latest commit has the version tag
    # shellcheck disable=SC2155
    local tag="$(git describe --exact-match HEAD)"
    if [ "${tag}" != "${RELEASE_VERSION}" ]; then
      log_error "Error: Expected HEAD to be tagged with ${RELEASE_VERSION}, but 'git describe --exact-match HEAD' reported: ${tag}"
      exit 1
    fi

    if [ "${IN_PLACE}" == 0 ]; then
      # Tried with `local branch=$(git branch -a --contains tags/"${RELEASE_VERSION}")`
      # so as to work with both current branch and main/release-3.X.
      # But got error below on current branch mode,
      # Error: Git tag v3.6.99 should be on branch '* (HEAD detached at pull/14860/merge)' but is on '* (HEAD detached from pull/14860/merge)'
      #
      # Verify the version tag is on the right branch
      # shellcheck disable=SC2155
      local branch=$(git for-each-ref --contains "${RELEASE_VERSION}" --format="%(refname)" 'refs/heads' | cut -d '/' -f 3)
      if [ "${branch}" != "${BRANCH}" ]; then
        log_error "Error: Git tag ${RELEASE_VERSION} should be on branch '${BRANCH}' but is on '${branch}'"
        exit 1
      fi
    fi
  fi

  log_callout "Verify the latest commit has the version tag"
  # Verify the latest commit has the version tag
  # shellcheck disable=SC2155
  local tag="$(git describe --exact-match HEAD)"
  if [ "${tag}" != "${RELEASE_VERSION}" ]; then
    log_error "Error: Expected HEAD to be tagged with ${RELEASE_VERSION}, but 'git describe --exact-match HEAD' reported: ${tag}"
    exit 1
  fi

  log_callout "Verify the work space is clean"
  # Verify the clean working tree
  # shellcheck disable=SC2155
  local diff="$(git diff HEAD --stat)"
  if [[ "${diff}" != '' ]]; then
    log_error "Error: Expected clean working tree, but 'git diff --stat' reported: ${diff}"
    exit 1
  fi

  # Build release.
  # TODO: check the release directory for all required build artifacts.
  if [ -d release ]; then
    log_warning "Skipping release build step. /release directory already exists."
  else
    log_callout "Building release..."
    # Check for old and new names of the release build script.
    # TODO: Move the release script into this on as a function?
    REPOSITORY=$(pwd) ./scripts/build-release.sh "${RELEASE_VERSION}"
  fi

  # Sanity checks.
  "./release/etcd-${RELEASE_VERSION}-$(go env GOOS)-amd64/etcd" --version | grep -q "etcd Version: ${VERSION}" || true
  "./release/etcd-${RELEASE_VERSION}-$(go env GOOS)-amd64/etcdctl" version | grep -q "etcdctl version: ${VERSION}" || true

  # Generate SHA256SUMS
  log_callout "Generating sha256sums of release artifacts."
  pushd ./release
  # shellcheck disable=SC2010
  ls . | grep -E '\.tar.gz$|\.zip$' | xargs shasum -a 256 > ./SHA256SUMS
  popd
  if [ -s ./release/SHA256SUMS ]; then
    cat ./release/SHA256SUMS
  else
    log_error "sha256sums is not valid. Aborting."
    exit 1
  fi

  # Upload artifacts.
  if [ "$DRY_RUN" = "true" ] || [ "${NO_UPLOAD}" == 1 ]; then
    log_callout "Skipping artifact upload to gs://etcd. --no-upload flat is set or DRY_RUN is true."
  else
    read -p "Upload etcd ${RELEASE_VERSION} release artifacts to gs://etcd [y/N]? " -r confirm
    [[ "${confirm,,}" == "y" ]] || exit 1
    gsutil -m cp ./release/SHA256SUMS "gs://etcd/${RELEASE_VERSION}/"
    gsutil -m cp ./release/*.zip "gs://etcd/${RELEASE_VERSION}/"
    gsutil -m cp ./release/*.tar.gz "gs://etcd/${RELEASE_VERSION}/"
    gsutil -m acl ch -u allUsers:R -r "gs://etcd/${RELEASE_VERSION}/"
  fi

  # Push images.
  if [ "$DRY_RUN" = "true" ] || [ "${NO_DOCKER_PUSH}" == 1 ]; then
    log_callout "Skipping docker push. --no-docker-push flat is set or DRY_RUN is true."
  else
    read -p "Publish etcd ${RELEASE_VERSION} docker images to quay.io [y/N]? " -r confirm
    [[ "${confirm,,}" == "y" ]] || exit 1

    echo "Pushing container images to quay.io ${RELEASE_VERSION}"
    docker push "quay.io/coreos/etcd:${RELEASE_VERSION}"

    echo "Pushing container images to gcr.io ${RELEASE_VERSION}"
    docker push "gcr.io/etcd-development/etcd:${RELEASE_VERSION}"

    for TARGET_ARCH in "-arm64" "-ppc64le"; do
      log_callout "Pushing container images to quay.io ${RELEASE_VERSION}${TARGET_ARCH}"
      docker push "quay.io/coreos/etcd:${RELEASE_VERSION}${TARGET_ARCH}"

      log_callout "Pushing container images to gcr.io ${RELEASE_VERSION}${TARGET_ARCH}"
      docker push "gcr.io/etcd-development/etcd:${RELEASE_VERSION}${TARGET_ARCH}"
    done
  fi

  ### Release validation
  mkdir -p downloads

  # Check image versions
  for IMAGE in "quay.io/coreos/etcd:${RELEASE_VERSION}" "gcr.io/etcd-development/etcd:${RELEASE_VERSION}"; do
    # shellcheck disable=SC2155
    local image_version=$(docker run --rm "${IMAGE}" etcd --version | grep "etcd Version" | awk -F: '{print $2}' | tr -d '[:space:]')
    if [ "${image_version}" != "${VERSION}" ]; then
      log_error "Check failed: etcd --version output for ${IMAGE} is incorrect: ${image_version}"
      exit 1
    fi
  done

  # Check gsutil binary versions
  # shellcheck disable=SC2155
  local BINARY_TGZ="etcd-${RELEASE_VERSION}-$(go env GOOS)-amd64.tar.gz"
  if [ "${DRY_RUN}" == "true" ] || [ "${NO_UPLOAD}" == 1 ]; then
    cp "./release/${BINARY_TGZ}" downloads
  else
    gsutil cp "gs://etcd/${RELEASE_VERSION}/${BINARY_TGZ}" downloads
  fi
  tar -zx -C downloads -f "downloads/${BINARY_TGZ}"
  # shellcheck disable=SC2155
  local binary_version=$("./downloads/etcd-${RELEASE_VERSION}-$(go env GOOS)-amd64/etcd" --version | grep "etcd Version" | awk -F: '{print $2}' | tr -d '[:space:]')
  if [ "${binary_version}" != "${VERSION}" ]; then
    log_error "Check failed: etcd --version output for ${BINARY_TGZ} from gs://etcd/${RELEASE_VERSION} is incorrect: ${binary_version}"
    exit 1
  fi

  if [ "${DRY_RUN}" == "true" ] || [ "${NO_GH_RELEASE}" == 1 ]; then
    log_warning ""
    log_warning "WARNING: Skipping creating GitHub release, --no-gh-release is set."
    log_warning "WARNING: If not running on DRY_MODE, please do the GitHub release manually."
    log_warning ""
  else
    local gh_repo
    local release_notes_temp_file
    local release_url
    local gh_release_args=()

    # For the main branch (v3.6), we should mark the release as a prerelease.
    # The release-3.5 (v3.5) branch, should be marked as latest. And release-3.4 (v3.4)
    # should be left without any additional mark (therefore, it doesn't need a special argument).
    if [ "${BRANCH}" = "main" ]; then
      gh_release_args=(--prerelease)
    elif [ "${BRANCH}" = "release-3.5" ]; then
      gh_release_args=(--latest)
    fi

    if [ "${REPOSITORY}" = "$(pwd)" ]; then
      gh_repo=$(git remote get-url origin)
    else
      gh_repo="${REPOSITORY}"
    fi

    gh_repo=$(echo "${gh_repo}" | sed 's/^[^@]\+@//' | sed 's/https\?:\/\///' | sed 's/\.git$//' | tr ':' '/')
    log_callout "Creating GitHub release for ${RELEASE_VERSION} on ${gh_repo}"

    release_notes_temp_file=$(mktemp)

    local release_version=${RELEASE_VERSION#v} # Remove the v prefix from the release version (i.e., v3.6.1 -> 3.6.1)
    local release_version_major_minor=${release_version%.*} # Remove the patch from the version (i.e., 3.6)
    local release_version_major=${release_version_major_minor%.*} # Extract the major (i.e., 3)
    local release_version_minor=${release_version_major_minor/*./} # Extract the minor (i.e., 6)

    # Disable sellcheck SC2016, the single quoted syntax for sed is intentional.
    # shellcheck disable=SC2016
    sed 's/${RELEASE_VERSION}/'"${RELEASE_VERSION}"'/g' ./scripts/release_notes.tpl.txt |
      sed 's/${RELEASE_VERSION_MAJOR_MINOR}/'"${release_version_major_minor}"'/g' |
      sed 's/${RELEASE_VERSION_MAJOR}/'"${release_version_major}"'/g' |
      sed 's/${RELEASE_VERSION_MINOR}/'"${release_version_minor}"'/g' > "${release_notes_temp_file}"

    # This condition may seem redundant because of the previous check, but it's
    # necessary because release-3.4 doesn't have the maybe_run function.
    if [ "${DRY_RUN}" == "false" ]; then
      if ! gh --repo "${gh_repo}" release view "${RELEASE_VERSION}" &>/dev/null; then
        gh release create "${RELEASE_VERSION}" \
            --repo "${gh_repo}" \
            --draft \
            --title "${RELEASE_VERSION}" \
            --notes-file "${release_notes_temp_file}" \
            "${gh_release_args[@]}"
      fi

      # Upload files one by one, as gh doesn't support passing globs as input.
      find ./release '(' -name '*.tar.gz' -o -name '*.zip' ')' -exec \
        gh --repo "${gh_repo}" release upload "${RELEASE_VERSION}" {} --clobber \;
      gh --repo "${gh_repo}" release upload "${RELEASE_VERSION}" ./release/SHA256SUMS --clobber

      release_url=$(gh --repo "${gh_repo}" release view "${RELEASE_VERSION}" --json url --jq '.url')

      log_warning ""
      log_warning "WARNING: The GitHub release for ${RELEASE_VERSION} has been created as a draft, please go to ${release_url} and release it."
      log_warning ""
    fi
  fi

  log_success "Success."
  exit 0
}

POSITIONAL=()
NO_UPLOAD=0
NO_DOCKER_PUSH=0
IN_PLACE=0
NO_GH_RELEASE=0

while test $# -gt 0; do
        case "$1" in
          -h|--help)
            shift
            help
            exit 0
            ;;
          --in-place)
            IN_PLACE=1
            shift
            ;;
          --no-upload)
            NO_UPLOAD=1
            shift
            ;;
          --no-docker-push)
            NO_DOCKER_PUSH=1
            shift
            ;;
          --no-gh-release)
            NO_GH_RELEASE=1
            shift
            ;;
          *)
            POSITIONAL+=("$1") # save it in an array for later
            shift # past argument
            ;;
        esac
done
set -- "${POSITIONAL[@]}" # restore positional parameters

if [[ ! $# -eq 1 ]]; then
  help
  exit 1
fi

# Note that we shouldn't upload artifacts in --in-place mode, so it
# must be called with DRY_RUN=true
if [ "${DRY_RUN}" != "true" ] && [ "${IN_PLACE}" == 1 ]; then
   log_error "--in-place should only be called with DRY_RUN=true"
   exit 1
fi

main "$1"
