#!/usr/bin/env bash
# Copyright 2025 The etcd Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
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
