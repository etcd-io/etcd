ARG GO_VERSION=1.24.6
ARG ARCH=amd64

FROM golang:$GO_VERSION AS build
WORKDIR /build
COPY . .

WORKDIR /build/tests
RUN go build -o /opt/antithesis/entrypoint/entrypoint -race ./antithesis/test-template/entrypoint/main.go
RUN go build -o /opt/antithesis/test/v1/robustness/singleton_driver_traffic -race ./antithesis/test-template/robustness/traffic/main.go
RUN go build -o /opt/antithesis/test/v1/robustness/finally_validation -race ./antithesis/test-template/robustness/finally/main.go

FROM ubuntu:24.04
COPY --from=build /opt/ /opt/
