FROM golang
ADD . /go/src/github.com/coreos/etcd
RUN go install github.com/coreos/etcd
EXPOSE 2379 2380
ENTRYPOINT ["etcd"]
