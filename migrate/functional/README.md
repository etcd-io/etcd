
etcd migration functional tests
=====

This functional test suite deploys a etcd cluster using processes, and asserts etcd is functioning properly.

Dependencies
------------

The test suite can only be run in linux system. It's recommended to run this in a virtual machine environment on CoreOS (e.g. using coreos-vagrant). The only dependency for the tests not provided on the CoreOS image is go.

Usage
-----

Set environment variables point to the respective binaries that are used to drive the actual tests:

```
$ export ETCD_V1_BIN=/path/to/v1_etcd
$ export ETCD_V2_BIN=/path/to/v2_etcd
$ export ETCDCTL_BIN=/path/to/etcdctl
```

Then the tests can be run:

```
$ go test github.com/coreos/etcd/migrate/functional
```
