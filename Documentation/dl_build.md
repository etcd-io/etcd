---
title: Download and build
weight: 1
---

## System requirements

The etcd performance benchmarks run etcd on 8 vCPU, 16GB RAM, 50GB SSD GCE instances, but any relatively modern machine with low latency storage and a few gigabytes of memory should suffice for most use cases. Applications with large v2 data stores will require more memory than a large v3 data store since data is kept in anonymous memory instead of memory mapped from a file. For running etcd on a cloud provider, see the [Example hardware configuration][example-hardware-configurations] documentation.

## Download the pre-built binary

The easiest way to get etcd is to use one of the pre-built release binaries which are available for OSX, Linux, Windows, appc, and Docker. Instructions for using these binaries are on the [GitHub releases page][github-release].

## Build the latest version

For those wanting to try the very latest version, build etcd from the `master` branch. [Go](https://golang.org/) version 1.9+ is required to build the latest version of etcd. To ensure etcd is built against well-tested libraries, etcd vendors its dependencies for official release binaries. However, etcd's vendoring is also optional to avoid potential import conflicts when embedding the etcd server or using the etcd client.

To build `etcd` from the `master` branch without a `GOPATH` using the official `build` script:

```sh
$ git clone https://github.com/etcd-io/etcd.git
$ cd etcd
$ ./build
```

To build a vendored `etcd` from the `master` branch via `go get`:

```sh
# GOPATH should be set
$ echo $GOPATH
/Users/example/go
$ go get -v go.etcd.io/etcd
$ go get -v go.etcd.io/etcd/etcdctl
```

## Test the installation

Check the etcd binary is built correctly by starting etcd and setting a key.

### Starting etcd

If etcd is built without using `go get`, run the following:

```sh
$ ./bin/etcd
```
If etcd is built using `go get`, run the following:

```sh
$ $GOPATH/bin/etcd
```

### Setting a key

Run the following:

```sh
$ ./bin/etcdctl put foo bar
OK
```

(or `$GOPATH/bin/etcdctl put foo bar` if etcdctl was installed with `go get`)

If OK is printed, then etcd is working!

[github-release]: https://github.com/etcd-io/etcd/releases/
[go]: https://golang.org/doc/install
[build-script]: ../build
[cmd-directory]: ../cmd
[example-hardware-configurations]: op-guide/hardware.md#example-hardware-configurations
