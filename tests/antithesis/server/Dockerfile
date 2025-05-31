ARG GO_VERSION=1.24.3
ARG ARCH=amd64

FROM golang:$GO_VERSION

# cloning etcd
ARG REF=main
RUN git clone --depth=1 https://github.com/etcd-io/etcd.git --branch=${REF} /etcd
WORKDIR /etcd

# setup go mod
RUN go mod download

# install instrumentor
RUN go get github.com/antithesishq/antithesis-sdk-go@a802e8810442e01d16b3e9df77d7ce3875e36e55 # v0.4.3
RUN go install github.com/antithesishq/antithesis-sdk-go/tools/antithesis-go-instrumentor@a802e8810442e01d16b3e9df77d7ce3875e36e55 # v0.4.3
RUN go mod tidy

# compile etcd server with instrumentor
RUN mkdir /etcd_instrumented
RUN `go env GOPATH`/bin/antithesis-go-instrumentor /etcd /etcd_instrumented
RUN rm -rf /etcd
RUN mkdir -p /symbols
RUN cp -r /etcd_instrumented/symbols/* /symbols

EXPOSE 2379 2380

# start etcd server
WORKDIR /etcd_instrumented/customer
# The instrumentation adds code and packages. Need go mod tidy for all modules before building
RUN go mod tidy || true
RUN for d in $(ls -d */); do (cd ${d} && go mod tidy || true); done
# The instrumentation also adds a new main file which clashes with dummy.go found in non release-3.4 branches
RUN [ -f "dummy.go" ] && sed -i 's/package main_test/package main/' dummy.go || true
RUN CGO_ENABLED=1 make build
CMD ["./bin/etcd"]
