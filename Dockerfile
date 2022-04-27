FROM golang:1.17.8 as build
RUN mkdir -p /go/go.etcd.io/etcd
WORKDIR /go/go.etcd.io/etcd
ADD go.mod go.mod
ADD go.sum go.sum
ADD api/go.mod api/go.mod
ADD api/go.sum api/go.sum
ADD client/pkg/go.mod client/pkg/go.mod
ADD client/pkg/go.sum client/pkg/go.sum
ADD client/v2/go.mod client/v2/go.mod
ADD client/v2/go.sum client/v2/go.sum
ADD client/v3/go.mod client/v3/go.mod
ADD client/v3/go.sum client/v3/go.sum
ADD etcdctl/go.mod etcdctl/go.mod
ADD etcdctl/go.sum etcdctl/go.sum
ADD etcdutl/go.mod etcdutl/go.mod
ADD etcdutl/go.sum etcdutl/go.sum
ADD pkg/go.mod pkg/go.mod
ADD pkg/go.sum pkg/go.sum
ADD raft/go.mod raft/go.mod
ADD raft/go.sum raft/go.sum
ADD server/go.mod server/go.mod
ADD server/go.sum server/go.sum
ADD tests/go.mod tests/go.mod
ADD tests/go.sum tests/go.sum
RUN go mod download
ADD . .
ARG GOARCH
ARG GOOS
RUN GOARCH=$GOARCH GOOS=$GOOS GO_LDFLAGS="-s" ./scripts/build.sh
FROM gcr.io/distroless/base-debian11

COPY --from=build /go/go.etcd.io/etcd/bin/etcd /usr/local/bin/
COPY --from=build /go/go.etcd.io/etcd/bin/etcdctl /usr/local/bin/
COPY --from=build /go/go.etcd.io/etcd/bin/etcdutl /usr/local/bin/

EXPOSE 2379 2380
# Define default command.
CMD ["/usr/local/bin/etcd"]
