#!/usr/bin/env bash
set -e
set -o pipefail

PROJ="go-systemd"
ORG_PATH="github.com/coreos"
REPO_PATH="${ORG_PATH}/${PROJ}"

PACKAGES="activation daemon dbus journal login1 machine1 sdjournal unit util import1"
EXAMPLES="activation listen udpconn"

function build_source {
    go build ./...
}

function build_tests {
    rm -rf ./test_bins ; mkdir -p ./test_bins
    for pkg in ${PACKAGES}; do
        echo "  - ${pkg}"
        go test -c -o ./test_bins/${pkg}.test ./${pkg}
    done
    for ex in ${EXAMPLES}; do
        echo "  - examples/${ex}"
        go build -o ./test_bins/${ex}.example ./examples/activation/${ex}.go
    done
}

function run_tests {
    pushd test_bins
    sudo -v
    for pkg in ${PACKAGES}; do
        echo "  - ${pkg}"
        sudo -E ./${pkg}.test -test.v
    done
    popd
    sudo rm -rf ./test_bins
}

function go_fmt {
    for pkg in ${PACKAGES}; do
        echo "  - ${pkg}"
        fmtRes=$(gofmt -l "./${pkg}")
        if [ -n "${fmtRes}" ]; then
            echo -e "gofmt checking failed:\n${fmtRes}"
            exit 255
        fi
    done
}

function go_vet {
    for pkg in ${PACKAGES}; do
        echo "  - ${pkg}"
        vetRes=$(go vet "./${pkg}")
        if [ -n "${vetRes}" ]; then
            echo -e "govet checking failed:\n${vetRes}"
            exit 254
        fi
    done
}

function license_check {
    licRes=$(for file in $(find . -type f -iname '*.go' ! -path './vendor/*'); do
  	             head -n3 "${file}" | grep -Eq "(Copyright|generated|GENERATED)" || echo -e "  ${file}"
  	         done;)
    if [ -n "${licRes}" ]; then
        echo -e "license header checking failed:\n${licRes}"
  	    exit 253
    fi
}

export GO15VENDOREXPERIMENT=1

subcommand="$1"
case "$subcommand" in
    "build_source" )
        echo "Building source..."
        build_source
        ;;

    "build_tests" )
        echo "Building tests..."
        build_tests
        ;;

    "run_tests" )
        echo "Running tests..."
        run_tests
        ;;

    "go_fmt" )
        echo "Checking gofmt..."
        go_fmt
        ;;

    "go_vet" )
        echo "Checking govet..."
        go_vet
        ;;

    "license_check" )
        echo "Checking licenses..."
        license_check
        ;;

    * )
        echo "Error: unrecognized subcommand."
        exit 1
    ;;
esac
