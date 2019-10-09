FROM ubuntu:18.04

RUN rm /bin/sh && ln -s /bin/bash /bin/sh
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections

RUN apt-get -y update \
  && apt-get -y install \
  build-essential \
  gcc \
  apt-utils \
  pkg-config \
  software-properties-common \
  apt-transport-https \
  libssl-dev \
  sudo \
  bash \
  curl \
  tar \
  git \
  netcat \
  bind9 \
  dnsutils \
  && apt-get -y update \
  && apt-get -y upgrade \
  && apt-get -y autoremove \
  && apt-get -y autoclean

ENV GOROOT /usr/local/go
ENV GOPATH /go
ENV PATH ${GOPATH}/bin:${GOROOT}/bin:${PATH}
ENV GO_VERSION REPLACE_ME_GO_VERSION
ENV GO_DOWNLOAD_URL https://storage.googleapis.com/golang
RUN rm -rf ${GOROOT} \
  && curl -s ${GO_DOWNLOAD_URL}/go${GO_VERSION}.linux-amd64.tar.gz | tar -v -C /usr/local/ -xz \
  && mkdir -p ${GOPATH}/src ${GOPATH}/bin \
  && go version \
  && go get -v -u github.com/mattn/goreman

RUN mkdir -p /var/bind /etc/bind
RUN chown root:bind /var/bind /etc/bind

ADD named.conf etcd.zone rdns.zone /etc/bind/
RUN chown root:bind /etc/bind/named.conf /etc/bind/etcd.zone /etc/bind/rdns.zone
ADD resolv.conf /etc/resolv.conf
