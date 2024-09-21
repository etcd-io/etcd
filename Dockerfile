ARG ARCH=amd64
FROM --platform=linux/${ARCH} gcr.io/distroless/static-debian12@sha256:95eb83a44a62c1c27e5f0b38d26085c486d71ece83dd64540b7209536bb13f6d

ADD etcd /usr/local/bin/
ADD etcdctl /usr/local/bin/
ADD etcdutl /usr/local/bin/

WORKDIR /var/etcd/
WORKDIR /var/lib/etcd/

EXPOSE 2379 2380

# Define default command.
CMD ["/usr/local/bin/etcd"]
