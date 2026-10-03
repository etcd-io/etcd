ARG GO_IMAGE_TAG
ARG ARCH=amd64

FROM golang:$GO_IMAGE_TAG AS build
WORKDIR /build
COPY . .

# Replacing math/rand with etcd's antithesis-sdk-go/random wrapper
# See https://antithesis.com/docs/configuration/the_antithesis_environment/?sid=8fce.35&sterm=random&marks=Random#random-devices
WORKDIR /build
RUN find . -path "./tests/antithesis" -prune -type f -o -name 'go.mod' -type f -execdir go mod edit -replace=math/rand=$(git rev-parse --show-toplevel)/tests/antithesis/pkg/rand \;
RUN find . -path "./tests/antithesis" -prune -type f -o -name 'go.mod' -type f -execdir go mod edit -replace=math/rand/v2=$(git rev-parse --show-toplevel)/tests/antithesis/pkg/rand/v2 \;
RUN find . -name 'go.mod' -type f -execdir go mod tidy \;

WORKDIR /build/tests
RUN go build -o /opt/antithesis/entrypoint/entrypoint -race ./antithesis/test-template/entrypoint/main.go
RUN go build -o /opt/antithesis/test/v1/robustness/singleton_driver_traffic -race ./antithesis/test-template/robustness/traffic/main.go
RUN go build -o /opt/antithesis/test/v1/robustness/finally_validation -race ./antithesis/test-template/robustness/finally/main.go

FROM ubuntu:24.04
COPY --from=build /opt/ /opt/
