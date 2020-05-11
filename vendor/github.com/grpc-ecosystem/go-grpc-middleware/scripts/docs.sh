#!/bin/bash
# Script that checks the code for errors.

GOBIN=${GOBIN:="$GOPATH/bin"}

function print_real_go_files {
    grep --files-without-match 'DO NOT EDIT!' $(find . -iname '*.go') --exclude=./vendor/*
}

function generate_markdown {
    echo "Generating Github markdown"
    oldpwd=$(pwd)
    for i in $(find . -iname 'doc.go' -not -path "*vendor/*"); do
        realdir=$(cd $(dirname ${i}) && pwd -P)
        package=${realdir##${GOPATH}/src/}
        echo "$package"
        cd ${dir}
        ${GOBIN}/godoc2ghmd -ex -file DOC.md ${package}
        ln -s DOC.md README.md 2> /dev/null # can fail
        cd ${oldpwd}
    done;
}

function generate {
    go get github.com/devnev/godoc2ghmd
    generate_markdown
    echo "returning $?"
}

function check {
    generate
    count=$(git diff --numstat | wc -l | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
    echo $count
    if [ "$count" = "0" ]; then
        return 0
    else
        echo "Your markdown docs seem to be out of sync with the package docs. Please run make and consult CONTRIBUTING.MD"
        return 1
    fi
}

"$@"