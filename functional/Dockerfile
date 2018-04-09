FROM fedora:28

RUN dnf check-update || true \
  && dnf install --assumeyes \
  git curl wget mercurial meld gcc gcc-c++ which \
  gcc automake autoconf dh-autoreconf libtool libtool-ltdl \
  tar unzip gzip \
  && dnf check-update || true \
  && dnf upgrade --assumeyes || true \
  && dnf autoremove --assumeyes || true \
  && dnf clean all || true

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
ADD . ${GOPATH}/src/github.com/coreos/etcd
ADD ./functional.yaml /functional.yaml

RUN go get -v github.com/coreos/gofail \
  && pushd ${GOPATH}/src/github.com/coreos/etcd \
  && GO_BUILD_FLAGS="-v" ./build \
  && mkdir -p /bin \
  && cp ./bin/etcd /bin/etcd \
  && cp ./bin/etcdctl /bin/etcdctl \
  && GO_BUILD_FLAGS="-v" FAILPOINTS=1 ./build \
  && cp ./bin/etcd /bin/etcd-failpoints \
  && ./functional/build \
  && cp ./bin/etcd-agent /bin/etcd-agent \
  && cp ./bin/etcd-proxy /bin/etcd-proxy \
  && cp ./bin/etcd-runner /bin/etcd-runner \
  && cp ./bin/etcd-tester /bin/etcd-tester \
  && go build -v -o /bin/benchmark ./tools/benchmark \
  && popd \
  && rm -rf ${GOPATH}/src/github.com/coreos/etcd