FROM alpine
RUN apk update 
RUN apk --update add iptables bash iproute2
ADD bin/etcd-agent /
ADD bin/etcd /
ADD bin/etcd-tester /
RUN mkdir /failure_archive
CMD ["./etcd-agent", "-etcd-path", "./etcd"]
