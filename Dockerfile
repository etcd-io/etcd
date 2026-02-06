FROM gcr.io/distroless/static-debian12@sha256:20bc6c0bc4d625a22a8fde3e55f6515709b32055ef8fb9cfbddaa06d1760f838

ARG TARGETARCH
ARG VERSION
ARG BUILD_DIR

ADD ${BUILD_DIR}/etcd-${VERSION}-linux-${TARGETARCH}/etcd /usr/local/bin/
ADD ${BUILD_DIR}/etcd-${VERSION}-linux-${TARGETARCH}/etcdctl /usr/local/bin/
ADD ${BUILD_DIR}/etcd-${VERSION}-linux-${TARGETARCH}/etcdutl /usr/local/bin/

WORKDIR /var/etcd/
WORKDIR /var/lib/etcd/

EXPOSE 2379 2380

# Define default command.
CMD ["/usr/local/bin/etcd"]
