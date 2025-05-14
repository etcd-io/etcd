ARG GO_VERSION
ARG ARCH=amd64

FROM golang:$GO_VERSION
WORKDIR /build
COPY . .

WORKDIR /build/tests
RUN go build -o /opt/antithesis/entrypoint/entrypoint -race ./antithesis/test-template/entrypoint/main.go
RUN go build -o /opt/antithesis/test/v1/robustness/singleton_driver_main -race ./antithesis/test-template/robustness/main.go
