FROM golang:1.9.1-stretch
LABEL Description="Image for etcd DNS testing"

RUN apt update -y \
  && apt install -y \
  bind9 \
  dnsutils

RUN mkdir /var/bind
RUN chown bind /var/bind
ADD Procfile.tls /Procfile.tls
ADD run.sh /run.sh

ADD named.conf etcd.zone rdns.zone /etc/bind/
ADD resolv.conf /etc/resolv.conf

RUN go get github.com/mattn/goreman
CMD ["/run.sh"]