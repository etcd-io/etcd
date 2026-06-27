#!/usr/bin/env bash
set -o errexit -o nounset -o pipefail

declare -A help

binary() {
    mkdir -p dist
    go build -o dist/gotestsum .
}

binary-static() {
    echo "building static binary: dist/gotestsum"
    CGO_ENABLED=0 binary
}

update-golden() {
    gotestsum -- ./... -update
}

lint() {
    golangci-lint run -v --config .project/golangci-lint.yml
}

go-mod-tidy() {
    go mod tidy
    git diff --stat --exit-code go.mod go.sum
}

help[shell]='Run a shell in a golang docker container.

Env vars:

GOLANG_VERSION - the docker image tag used to build the image.
'
shell() {
    local image; image="$(_docker-build-dev)"
    docker run \
        --tty --interactive --rm \
        -v "$PWD:/work" \
        -v ~/.cache/go-build:/root/.cache/go-build \
        -v ~/go/pkg/mod:/go/pkg/mod \
        -w /work \
        "$image" \
        "${@-bash}"
}

_docker-build-dev() {
    set -e
    local idfile=".plsdo/docker-build-dev-image-id-${GOLANG_VERSION-default}"
    local dockerfile=.project/Dockerfile
    local tag=gotest.tools/gotestsum/builder
    if [ -f "$idfile" ] && [ "$dockerfile" -ot "$idfile" ]; then
        cat "$idfile"
        return 0
    fi

    mkdir -p .plsdo
    >&2 docker build \
        --iidfile "$idfile"  \
        --file "$dockerfile" \
        --build-arg "UID=$UID" \
        --build-arg GOLANG_VERSION \
        --target "dev" \
        .plsdo
    cat "$idfile"
}

help[godoc]="Run godoc locally to preview package documentation."
godoc() {
    local url; url="http://localhost:6060/pkg/$(go list)/"
    command -v xdg-open && xdg-open "$url" &
    command -v open && open "$url" &
    command godoc -http=:6060
}

help[list]="Print the list of tasks"
list() {
    declare -F | awk '{print $3}' | grep -v '^_'
}

_plsdo_help() {
    local topic="${1-}"
    # print help for the topic
    if [ -n "$topic" ]; then
        if ! command -v "$topic" > /dev/null ; then
            >&2 echo "No such task: $topic"
            return 1
        fi

        printf "\nUsage:\n  %s %s\n\n%s\n" "$0" "$topic" "${help[$topic]-}"
        return 0
    fi

    # print list of tasks and their help line.
    [ -n "${banner-}" ] && echo "$banner" && echo
    for i in $(list); do
        printf "%-12s\t%s\n" "$i" "${help[$i]-}" | head -1
    done
}

_plsdo_run() {
    case "${1-}" in
    ""|help)
        _plsdo_help "${2-}" ;;
    *)
        "$@" ;;
    esac
}

_plsdo_run "$@"
