FROM golang:1.8.4-stretch

RUN apt-get -y update
RUN apt-get -y install \
  netcat \
  libaspell-dev \
  libhunspell-dev \
  hunspell-en-us \
  aspell-en \
  shellcheck

RUN mkdir -p ${GOPATH}/src/github.com/coreos/etcd
WORKDIR ${GOPATH}/src/github.com/coreos/etcd

ADD ./scripts/install-marker.sh ./scripts/install-marker.sh

RUN go get -v -u -tags spell github.com/chzchzchz/goword \
  && go get -v -u github.com/coreos/license-bill-of-materials \
  && go get -v -u honnef.co/go/tools/cmd/gosimple \
  && go get -v -u honnef.co/go/tools/cmd/unused \
  && go get -v -u honnef.co/go/tools/cmd/staticcheck \
  && go get -v -u github.com/wadey/gocovmerge \
  && ./scripts/install-marker.sh amd64
