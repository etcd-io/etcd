FROM gcr.io/distroless/static-debian12@sha256:9c346e4be81b5ca7ff31a0d89eaeade58b0f95cfd3baed1f36083ddb47ca3160

ARG TARGETARCH
ARG VERSION
ARG BUILDDIR

ADD ${BUILDDIR}/etcd-${VERSION}-linux-${TARGETARCH}/etcd /usr/local/bin/
ADD ${BUILDDIR}/etcd-${VERSION}-linux-${TARGETARCH}/etcdctl /usr/local/bin/
ADD ${BUILDDIR}/etcd-${VERSION}-linux-${TARGETARCH}/etcdutl /usr/local/bin/

WORKDIR /var/etcd/
WORKDIR /var/lib/etcd/

EXPOSE 2379 2380

# Define default command.
CMD ["/usr/local/bin/etcd"]
