ARG GO_VERSION=1.24.2
ARG ARCH=amd64

FROM golang:$GO_VERSION
WORKDIR /build
COPY . .

WORKDIR /build/tests
RUN go build -o /opt/antithesis/entrypoint/entrypoint -race ./antithesis/test-template/entrypoint/entrypoint.go
RUN go build -o /opt/antithesis/test/v1/main/serial_driver_delete_keys -race ./antithesis/test-template/go-delete-keys/serial_driver_delete_keys.go
