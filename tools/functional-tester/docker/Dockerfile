FROM alpine
RUN apk update 
RUN apk add -v iptables sudo
ADD bin/etcd /
ADD bin/functional-tester /
RUN mkdir /failure_archive
CMD ["./functional-tester", "agent", "--etcd-path", "./etcd"]
