#!/usr/bin/env bash
# This scripts is automatically run by CI to prevent pull requests missing running genproto.sh
# after changing *.proto file.

set -o errexit
set -o nounset
set -o pipefail

tmpWorkDir=$(mktemp -d -t 'twd.XXXXXX')
mkdir "$tmpWorkDir/etcd"
tmpWorkDir="$tmpWorkDir/etcd"
cp -r . "$tmpWorkDir"
pushd "$tmpWorkDir"
git add -A
git commit -m init || true # maybe fail because nothing to commit 
./scripts/genproto.sh
diff=$(git diff --numstat | awk '{print $3}')
popd
if [ -z "$diff" ]; then
  echo "PASSED genproto-verification!"
  exit 0
fi
echo "Failed genproto-verification!" >&2
printf "* Found changed files:\n%s\n" "$diff" >&2
echo "* Please rerun genproto.sh after changing *.proto file" >&2
echo "* Run ./scripts/genproto.sh" >&2
exit 1
