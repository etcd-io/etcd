ARG ARCH=amd64
FROM --platform=linux/${ARCH} gcr.io/distroless/static-debian12@sha256:2eb0c793360d26aace9b88fb9cf1a4f680140e7fb7d68d86de1fe63dbc1a7660

ADD etcd /usr/local/bin/
ADD etcdctl /usr/local/bin/
ADD etcdutl /usr/local/bin/

WORKDIR /var/etcd/
WORKDIR /var/lib/etcd/

EXPOSE 2379 2380

# Define default command.
CMD ["/usr/local/bin/etcd"]
