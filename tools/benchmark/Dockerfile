ARG GO_VERSION=1.26.2

FROM golang:${GO_VERSION} AS build
WORKDIR /workspace
COPY . .
RUN GO_BUILD_FLAGS="-v -mod=readonly" ./scripts/build.sh
RUN GO_BUILD_FLAGS="-v -mod=readonly" ./scripts/build_tools.sh

FROM gcr.io/distroless/static-debian12@sha256:20bc6c0bc4d625a22a8fde3e55f6515709b32055ef8fb9cfbddaa06d1760f838 AS etcd
COPY --from=build /workspace/bin/etcd /usr/local/bin/etcd
COPY --from=build /workspace/bin/etcdctl /usr/local/bin/etcdctl
COPY --from=build /workspace/bin/etcdutl /usr/local/bin/etcdutl
WORKDIR /var/etcd/
WORKDIR /var/lib/etcd
EXPOSE 2379 2380
ENTRYPOINT ["/usr/local/bin/etcd"]

FROM gcr.io/distroless/static-debian12@sha256:20bc6c0bc4d625a22a8fde3e55f6515709b32055ef8fb9cfbddaa06d1760f838 AS benchmark
COPY --from=build /workspace/bin/tools/benchmark /usr/local/bin/benchmark
WORKDIR /var/etcd/benchmark
ENTRYPOINT ["/usr/local/bin/benchmark"]
