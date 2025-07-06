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
ENV GO_VERSION 1.14.3
ENV GO_DOWNLOAD_URL https://storage.googleapis.com/golang
RUN rm -rf ${GOROOT} \
  && curl -s ${GO_DOWNLOAD_URL}/go${GO_VERSION}.linux-amd64.tar.gz | tar -v -C /usr/local/ -xz \
  && mkdir -p ${GOPATH}/src ${GOPATH}/bin \
  && go version

RUN mkdir -p ${GOPATH}/src/go.etcd.io/etcd
ADD . ${GOPATH}/src/go.etcd.io/etcd
ADD ./tests/functional/functional.yaml /functional.yaml

RUN go get -v go.etcd.io/gofail \
  && pushd ${GOPATH}/src/go.etcd.io/etcd \
  && GO_BUILD_FLAGS="-v" ./build.sh \
  && mkdir -p /bin \
  && cp ./bin/etcd /bin/etcd \
  && cp ./bin/etcdctl /bin/etcdctl \
  && GO_BUILD_FLAGS="-v" FAILPOINTS=1 ./build.sh \
  && cp ./bin/etcd /bin/etcd-failpoints \
  && ./tests/functional/build \
  && cp ./bin/etcd-agent /bin/etcd-agent \
  && cp ./bin/etcd-proxy /bin/etcd-proxy \
  && cp ./bin/etcd-runner /bin/etcd-runner \
  && cp ./bin/etcd-tester /bin/etcd-tester \
  && go build -v -o /bin/benchmark ./tools/benchmark \
  && popd \
  && rm -rf ${GOPATH}/src/go.etcd.io/etcd
