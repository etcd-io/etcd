#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ "./tests.sh" ]]; then
  echo "must be run from repository root"
  exit 255
fi

IGNORE_PKGS="(vendor)"
TESTS=`find . -name \*_test.go | while read a; do dirname $a; done | sort | uniq | egrep -v "$IGNORE_PKGS"`

echo "Checking gofmt..." $TESTS
fmtRes=$(gofmt -l -s -d $TESTS)
if [[ "${fmtRes}" ]]; then
  echo -e "gofmt checking failed:\n${fmtRes}"
  exit 255
fi

echo "Checking govet..." $TESTS
vetRes=$(go vet $TESTS)
if [[ "${vetRes}" ]]; then
  echo -e "govet checking failed:\n${vetRes}"
  exit 255
fi

<<COMMENT
OS=linux  GOARCH=amd64 ETCD_VERSIONS="v3.2.24 v3.3.9" ./tests.sh
OS=darwin GOARCH=amd64 ETCD_VERSIONS="v3.2.24 v3.3.9" ./tests.sh
COMMENT
echo "Downloading etcd releases..."
OS=linux
ext="tar.gz"
if [[ $(uname) = "Darwin" ]]; then
  echo "Running locally with MacOS"
  OS=darwin
  ext="zip"
fi

if [ -z "${GOARCH}" ]; then
  GOARCH=amd64;
fi

for ver in ${ETCD_VERSIONS}; do
  mkdir -p ./bin/
  rm -f ./bin/etcd-${ver}

  file="etcd-${ver}-${OS}-${GOARCH}.${ext}"
  echo "Downloading ${file}"

  set +e
  curl --fail -L https://github.com/etcd-io/etcd/releases/download/${ver}/${file} -o /tmp/${file}
  result=$?
  set -e
  case $result in
    0) ;;
    *) echo "FAIL with" ${result}
       exit $result
       ;;
  esac

  if [[ ${OS} = "linux" ]]; then
    rm -rf /tmp/etcd
    tar xzvf /tmp/${file} -C /tmp/ --strip-components=1
    mv /tmp/etcd ./bin/etcd-${ver}
    mv /tmp/etcdctl ./bin/etcdctl-${ver}
  elif [[ ${OS} = "darwin" ]]; then
    rm -rf /tmp/etcd-${ver}-${OS}-${GOARCH}
    unzip /tmp/${file} -d /tmp
    mv /tmp/etcd-${ver}-${OS}-${GOARCH}/etcd ./bin/etcd-${ver}
    mv /tmp/etcd-${ver}-${OS}-${GOARCH}/etcdctl ./bin/etcdctl-${ver}
  fi

  ./bin/etcd-${ver} --version
  ETCDCTL_API=3 ./bin/etcdctl-${ver} version
done

echo "Building discovery.etcd.io..."
go build -v -o ./bin/discovery .

echo "Running tests..." $TESTS
EXPECT_DEBUG=1 go test -v $TESTS;
EXPECT_DEBUG=1 go test -v -race $TESTS;
