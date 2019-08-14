#!/bin/bash

set -e

if [[ "$TRAVIS_GO_VERSION" =~ ^1.\12\. ]] && [[ "$TRAVIS_OS_NAME" == "linux" ]]; then
    git clone https://github.com/dgsb/gox.git /tmp/gox
    pushd /tmp/gox
    git checkout new_master
    go build ./
    popd
fi
