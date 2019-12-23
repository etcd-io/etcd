#! /bin/bash

function usage {
  echo "${0} -t tag_name"
}

function tag_exists {
  local tag="${1}"
  git --no-pager tag | grep -q "${tag}"'$'
  return $?
}

while getopts "t:h" option; do
  case "${option}" in
    t)
      tag=${OPTARG}
    ;;
    h)
      usage
      exit 0
    ;;
    \?)
      usage
      exit 1
    ;;
  esac
done


if [ -z "${tag}" ]; then
  usage
  exit 1
fi


REPO_ROOT=$(git -C "$(dirname $( readlink -f ${BASH_SOURCE[0]}))" rev-parse --show-toplevel)
pushd ${REPO_ROOT} > /dev/null 2>&1
if ! tag_exists "${tag}"; then
  echo "tag ${tag} does not exist"
  echo "use 'git tag' to list available ones"
  exit 1
else
  git checkout tags/$tag > /dev/null 2>&1
  echo -n v0.0.0-
  TZ=UTC git --no-pager show \
    --quiet \
    --abbrev=12 \
    --date='format-local:%Y%m%d%H%M%S' \
    --format="%cd-%h"
  git checkout - > /dev/null 2>&1
fi
popd > /dev/null 2>&1
