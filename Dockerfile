FROM gcr.io/distroless/static-debian12@sha256:cd64bec9cec257044ce3a8dd3620cf83b387920100332f2b041f19c4d2febf93

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
