ARG GO_VERSION=1.24.2
ARG ARCH=amd64

FROM golang:$GO_VERSION AS builder
WORKDIR /build
COPY . .

WORKDIR /build/tests
RUN go build -o /opt/entrypoint -race ./antithesis/test-template/entrypoint/entrypoint.go
RUN go build -o /opt/serial_driver_delete_keys -race ./antithesis/test-template/go-delete-keys/serial_driver_delete_keys.go

FROM --platform=linux/${ARCH} gcr.io/distroless/static-debian12@sha256:3d0f463de06b7ddff27684ec3bfd0b54a425149d0f8685308b1fdf297b0265e9
COPY --from=builder /opt/entrypoint /opt/antithesis/entrypoint/entrypoint
COPY --from=builder /opt/serial_driver_delete_keys /opt/antithesis/test/v1/main/serial_driver_delete_keys
