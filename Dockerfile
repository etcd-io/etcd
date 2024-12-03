ARG ARCH=amd64
FROM --platform=linux/${ARCH} gcr.io/distroless/static-debian12@sha256:5c7e2b465ac6a2a4e5f4f7f722ce43b147dabe87cb21ac6c4007ae5178a1fa58

ADD etcd /usr/local/bin/
ADD etcdctl /usr/local/bin/
ADD etcdutl /usr/local/bin/

WORKDIR /var/etcd/
WORKDIR /var/lib/etcd/

EXPOSE 2379 2380

# Define default command.
CMD ["/usr/local/bin/etcd"]
