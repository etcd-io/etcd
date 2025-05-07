#!/usr/bin/env bash

ROOT_MODULE="go.etcd.io/etcd"

if [[ "$(go list)" != "${ROOT_MODULE}" ]]; then
  echo "must be run from '${ROOT_MODULE}' module directory"
  exit 255
fi

####   Convenient IO methods #####

COLOR_RED='\033[0;31m'
COLOR_ORANGE='\033[0;33m'
COLOR_GREEN='\033[0;32m'
COLOR_LIGHTCYAN='\033[0;36m'
COLOR_BLUE='\033[0;94m'
COLOR_BOLD='\033[1m'
COLOR_NONE='\033[0m' # No Color

function log_error {
  >&2 echo -n -e "${COLOR_BOLD}${COLOR_RED}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

function log_warning {
  >&2 echo -n -e "${COLOR_ORANGE}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

function log_callout {
  >&2 echo -n -e "${COLOR_LIGHTCYAN}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

function log_cmd {
  >&2 echo -n -e "${COLOR_BLUE}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

function log_success {
  >&2 echo -n -e "${COLOR_GREEN}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

function log_info {
  >&2 echo -n -e "${COLOR_NONE}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

# The version present in the .go-verion is the default version that test and build scripts will use.
# However, it is possible to control the version that should be used with the help of env vars:
# - FORCE_HOST_GO: if set to a non-empty value, use the version of go installed in system's $PATH.
# - GO_VERSION: desired version of go to be used, might differ from what is present in .go-version.
#               If empty, the value defaults to the version in .go-version.
function determine_go_version {
  # Borrowing from how Kubernetes does this:
  #  https://github.com/kubernetes/kubernetes/blob/17854f0e0a153b06f9d0db096e2cd8ab2fa89c11/hack/lib/golang.sh#L510-L520
  #
  # default GO_VERSION to content of .go-version
  GO_VERSION="${GO_VERSION:-"$(cat "./.go-version")"}"
  if [ "${GOTOOLCHAIN:-auto}" != 'auto' ]; then
    # no-op, just respect GOTOOLCHAIN
    :
  elif [ -n "${FORCE_HOST_GO:-}" ]; then
    export GOTOOLCHAIN='local'
  else
    GOTOOLCHAIN="go${GO_VERSION}"
    export GOTOOLCHAIN
  fi
}

determine_go_version
