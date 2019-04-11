FROM golang:1.11.3
MAINTAINER "etcd Authors"
EXPOSE 8087

COPY . /go/src/go.etcd.io/etcd/discoveryserver
RUN go install -v go.etcd.io/etcd/discoveryserver

CMD ["discoveryserver"]
