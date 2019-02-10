#!/usr/bin/env bash
#
# Generate coverage HTML for a package
# e.g. PKG=./unit ./tests/cover.test.bash
#
set -e

if ! [[ "$0" =~ "tests/cover.test.bash" ]]; then
  echo "must be run from repository root"
  exit 255
fi

if [ -z "$PKG" ]; then
  echo "cover only works with a single package, sorry"
  exit 255
fi

COVEROUT="coverage"

if ! [ -d "$COVEROUT" ]; then
	mkdir "$COVEROUT"
fi

# strip leading dot/slash and trailing slash and sanitize other slashes
# e.g. ./etcdserver/etcdhttp/ ==> etcdserver_etcdhttp
COVERPKG=${PKG/#./}
COVERPKG=${COVERPKG/#\//}
COVERPKG=${COVERPKG/%\//}
COVERPKG=${COVERPKG//\//_}

# generate arg for "go test"
export COVER="-coverprofile ${COVEROUT}/${COVERPKG}.out"

source ./test

go tool cover -html=${COVEROUT}/${COVERPKG}.out
