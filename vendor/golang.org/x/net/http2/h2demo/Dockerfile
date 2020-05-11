# Copyright 2018 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM golang:1.12 AS build
LABEL maintainer "golang-dev@googlegroups.com"

ENV GO111MODULE=on
ENV GOPROXY=https://proxy.golang.org

RUN mkdir /gocache
ENV GOCACHE /gocache

COPY go.mod /go/src/golang.org/x/build/go.mod
COPY go.sum /go/src/golang.org/x/build/go.sum

# Optimization for iterative docker build speed, not necessary for correctness:
# TODO: write a tool to make writing Go module-friendly Dockerfiles easier.
RUN go install cloud.google.com/go/storage
RUN go install golang.org/x/build/autocertcache
RUN go install go4.org/syncutil/singleflight
RUN go install golang.org/x/crypto/acme/autocert

COPY . /go/src/golang.org/x/net

# Install binary to /go/bin:
WORKDIR /go/src/golang.org/x/net/http2/h2demo
RUN go install -tags "h2demo netgo"


FROM debian:stretch
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates netbase \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /go/bin/h2demo /h2demo

