FROM gcr.io/distroless/static-debian12@sha256:d75cdd72874d4790092fcb1b058493ecf6bb5bf2b2b897045b00ff01d91843f2

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
