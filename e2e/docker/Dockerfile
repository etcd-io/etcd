FROM golang:1.8.3-stretch
LABEL Description="Image for etcd DNS testing"
RUN apt update -y
RUN go get github.com/mattn/goreman
RUN apt install -y bind9
RUN mkdir /var/bind
RUN chown bind /var/bind
ADD Procfile.tls /Procfile.tls
ADD run.sh /run.sh
ADD named.conf etcd.zone rdns.zone /etc/bind/
ADD resolv.conf /etc/resolv.conf
CMD ["/run.sh"]