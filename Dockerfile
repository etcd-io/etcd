ARG ARCH=amd64
FROM --platform=linux/${ARCH} gcr.io/distroless/static-debian12@sha256:b033683de7de51d8cce5aa4b47c1b9906786f6256017ca8b17b2551947fcf6d8

ADD etcd /usr/local/bin/
ADD etcdctl /usr/local/bin/
ADD etcdutl /usr/local/bin/

WORKDIR /var/etcd/
WORKDIR /var/lib/etcd/

EXPOSE 2379 2380

# Define default command.
CMD ["/usr/local/bin/etcd"]
