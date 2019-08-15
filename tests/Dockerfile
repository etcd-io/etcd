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
  wget \
  tar \
  git \
  netcat \
  libaspell-dev \
  libhunspell-dev \
  hunspell-en-us \
  aspell-en \
  shellcheck \
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
  && go version

RUN mkdir -p ${GOPATH}/src/github.com/coreos/etcd
WORKDIR ${GOPATH}/src/github.com/coreos/etcd
