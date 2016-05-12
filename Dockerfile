FROM alpine:latest
RUN apk update
RUN apk --update add bash

# in case we need to run functional-tester
RUN apk --update add iptables iproute2

ADD bin/etcd /usr/local/bin/
ADD bin/etcdctl /usr/local/bin/
RUN mkdir -p /var/etcd/

EXPOSE 2379 2380

# Define default command.
CMD ["bash"]
