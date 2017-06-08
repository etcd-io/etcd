FROM alpine
RUN apk update
RUN apk --update add iptables bash iproute2

FROM golang
RUN go get -v -u github.com/coreos/etcd
RUN go get -v -u github.com/coreos/etcd/tools/functional-tester/etcd-agent

EXPOSE 2378 2379 2380 9027
ENTRYPOINT /go/bin/etcd-agent
