ARG GO_VERSION=1.24.2
ARG ARCH=amd64

FROM golang:$GO_VERSION
WORKDIR /build
COPY . .

WORKDIR /build/tests
RUN go build -o /opt/antithesis/entrypoint/entrypoint -race ./antithesis/test-template/entrypoint/main.go
RUN go build -o /opt/antithesis/test/v1/main/robustness -race ./antithesis/test-template/robustness/main.go
