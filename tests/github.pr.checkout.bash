#!/usr/bin/env bash

if ! [[ "$0" =~ "tests/github.pr.checkout.bash" ]]; then
  echo "must be run from repository root"
  exit 255
fi

printf "\n"
printf "\n"
printf "\n"
printenv
printf "\n"
printf "\n"
printf "\n"

mkdir -p ${GOPATH}/src/github.com/etcd-io/
cd ${GOPATH}/src/github.com/etcd-io/
git clone https://github.com/etcd-io/etcd.git

cd ${GOPATH}/src/github.com/etcd-io/etcd
if [[ "${PULL_NUMBER}" ]]; then
  echo 'git fetching:' pull/${PULL_NUMBER}/head 'to test branch'
  git fetch origin pull/${PULL_NUMBER}/head:test
fi

# TODO: not work?
git remote -v

git branch
git checkout test
git log --pretty=oneline -10

printf "\n"
printf "\n"
printf "\n"
