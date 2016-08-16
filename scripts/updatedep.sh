#!/usr/bin/env bash

# A script for updating godep dependencies for the vendored directory /cmd/
# without pulling in etcd itself as a dependency.
#
# update depedency
# 1. edit glide.yaml with version, git SHA
# 2. run ./scripts/updatedep.sh
# 3. it automatically detects new git SHA, and vendors updates to cmd/vendor directory
#
# add depedency
# 1. run ./scripts/updatedep.sh github.com/USER/PROJECT#^1.0.0
#        OR
#        ./scripts/updatedep.sh github.com/USER/PROJECT#9b772b54b3bf0be1eec083c9669766a56332559a
# 2. make sure glide.yaml and glide.lock are updated

if ! [[ "$0" =~ "scripts/updatedep.sh" ]]; then
	echo "must be run from repository root"
	exit 255
fi

rm -rf vendor
mv cmd/vendor vendor

# TODO: glide doesn't play well with symlink
echo "manually deleting etcd-repo symlink in vendor"
rm -f vendor/github.com/coreos/etcd

GLIDE_ROOT=$GOPATH/src/github.com/Masterminds/glide
go get -v -u github.com/Masterminds/glide
go get -v -u github.com/sgotti/glide-vc
GLIDE_ROOT=$GOPATH/src/github.com/Masterminds/glide
GLIDE_SHA=3e49dce57f4a3a1e9bc55475065235766000d2f0
pushd "${GLIDE_ROOT}"
	git reset --hard ${GLIDE_SHA}
	go install
popd

if [ -n "$1" ]; then
	echo "glide get on $(echo $1)"
	glide --verbose get --strip-vendor --strip-vcs --update-vendored --skip-test $1
else
	echo "glide update on *"
	glide --verbose update --delete --strip-vendor --strip-vcs --update-vendored --skip-test
fi;

echo "removing test files"
glide vc --only-code --no-tests

mv vendor cmd/

echo "recreating symlink to etcd"
ln -s ../../../../ cmd/vendor/github.com/coreos/etcd

echo "done"

