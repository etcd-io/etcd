FROM golang:1.9.1-stretch
LABEL Description="Image for etcd DNS SRV testing"

RUN apt update -y \
  && apt install -y \
  bind9 \
  dnsutils

RUN mkdir /var/bind
RUN chown bind /var/bind
ADD Procfile /Procfile
ADD run.sh /run.sh

ADD etcd.zone named.conf /etc/bind/
ADD resolv.conf /etc/resolv.conf

RUN go get github.com/mattn/goreman
CMD ["/run.sh"]
