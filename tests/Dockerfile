FROM fedora:28

RUN dnf check-update || true \
  && dnf install --assumeyes \
  git curl wget mercurial meld gcc gcc-c++ which \
  gcc automake autoconf dh-autoreconf libtool libtool-ltdl \
  tar unzip gzip \
  aspell-devel aspell-en hunspell hunspell-devel hunspell-en hunspell-en-US ShellCheck nc || true \
  && dnf check-update || true \
  && dnf upgrade --assumeyes || true \
  && dnf autoremove --assumeyes || true \
  && dnf clean all || true \
  && dnf reinstall which || true

ENV GOROOT /usr/local/go
ENV GOPATH /go
ENV PATH ${GOPATH}/bin:${GOROOT}/bin:${PATH}
ENV GO_VERSION 1.10.1
ENV GO_DOWNLOAD_URL https://storage.googleapis.com/golang
RUN rm -rf ${GOROOT} \
  && curl -s ${GO_DOWNLOAD_URL}/go${GO_VERSION}.linux-amd64.tar.gz | tar -v -C /usr/local/ -xz \
  && mkdir -p ${GOPATH}/src ${GOPATH}/bin \
  && go version

RUN mkdir -p ${GOPATH}/src/github.com/coreos/etcd
WORKDIR ${GOPATH}/src/github.com/coreos/etcd

ADD ./scripts/install-marker.sh /tmp/install-marker.sh

# manually link "goword" dependency
# ldconfig -v | grep hunspell
RUN ln -s /lib64/libhunspell-1.6.so /lib64/libhunspell.so

RUN go get -v -u -tags spell github.com/chzchzchz/goword \
  && go get -v -u github.com/coreos/license-bill-of-materials \
  && go get -v -u honnef.co/go/tools/cmd/gosimple \
  && go get -v -u honnef.co/go/tools/cmd/unused \
  && go get -v -u honnef.co/go/tools/cmd/staticcheck \
  && go get -v -u github.com/gyuho/gocovmerge \
  && go get -v -u github.com/gordonklaus/ineffassign \
  && go get -v -u github.com/alexkohler/nakedret \
  && /tmp/install-marker.sh amd64 \
  && rm -f /tmp/install-marker.sh \
  && curl -s https://codecov.io/bash >/codecov \
  && chmod 700 /codecov
