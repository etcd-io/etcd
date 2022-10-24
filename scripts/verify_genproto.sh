#!/usr/bin/env bash
# This scripts is automatically run by CI to prevent pull requests missing running genproto.sh
# after changing *.proto file.

set -o errexit
set -o nounset
set -o pipefail

./scripts/genproto.sh
diff=$(git diff --name-only | grep -c ".pb.")
if [ "$diff" -eq 0 ]; then
  echo "PASSED genproto-verification!"
  exit 0
fi
echo "Failed genproto-verification!" >&2
echo "* Found changed files $(git diff --name-only | grep '.pb.')" >&2
echo "* Please rerun genproto.sh after changing *.proto file" >&2
echo "* Run ./scripts/genproto.sh" >&2
exit 1
