#!/bin/sh -e

SYSTEMS=(windows linux freebsd darwin)
ARCHS=(amd64 386)
ROOT="$GOPATH/src/honnef.co/go/tools"

rev="$1"
if [ -z "$rev" ]; then
    echo "Usage: $0 <version>"
    exit 1
fi


mkdir "$rev"
d=$(realpath "$rev")

wrk=$(mktemp -d)
trap "{ rm -rf \"$wrk\"; }" EXIT
cd "$wrk"

go mod init foo
GO111MODULE=on go get -d honnef.co/go/tools/cmd/staticcheck@"$rev"

for os in ${SYSTEMS[@]}; do
    for arch in ${ARCHS[@]}; do
        echo "Building GOOS=$os GOARCH=$arch..."
        exe="staticcheck"
        if [ $os = "windows" ]; then
            exe="${exe}.exe"
        fi
        target="staticcheck_${os}_${arch}"

        rm -rf "$d/staticcheck"
        mkdir "$d/staticcheck"
        cp "$ROOT/LICENSE" "$ROOT/LICENSE-THIRD-PARTY" "$d/staticcheck"
        CGO_ENABLED=0 GOOS=$os GOARCH=$arch GO111MODULE=on go build -o "$d/staticcheck/$exe" honnef.co/go/tools/cmd/staticcheck
        (
            cd "$d"
            tar -czf "$target.tar.gz" staticcheck
            sha256sum "$target.tar.gz" > "$target.tar.gz.sha256"
        )
    done
done

(
    cd "$d"
    sha256sum -c --strict *.sha256
)
