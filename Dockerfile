FROM alpine:latest

ADD bin/etcd /usr/local/bin/
ADD bin/etcdctl /usr/local/bin/
RUN mkdir -p /var/etcd/

EXPOSE 2379 2380

# Define default command.
CMD ["/usr/local/bin/etcd"]
