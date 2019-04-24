# discovery.etcd.io

[![Docker Repository on Quay](https://quay.io/repository/etcd/discoveryserver/status "Docker Repository on Quay")](https://quay.io/repository/etcd/discoveryserver)

This code powers the public service at https://discovery.etcd.io. The API is
documented in the [etcd clustering documentation](https://github.com/coreos/etcd/blob/master/Documentation/dev-internal/discovery_protocol.md#public-discovery-service).

# Configuration

The service has three configuration options, and can be configured with either
runtime arguments or environment variables.

* `--addr` / `DISC_ADDR`: the address to run the service on, including port.
* `--host` / `DISC_HOST`: the host url to prepend to `/new` requests.
* `--etcd` / `DISC_ETCD`: the url of the etcd endpoint backing the instance.

# Production Configuration

See https://github.com/etcd-io/discovery.etcd.io for how the service is ran on Kubernetes

# History

This package implements a super minimal etcd v2 API built on the currently
incomplete [v2v3 package]. We need to do this because the public etcd discovery
service has been running on the inefficient v2 storage engine and causing
operational burden.

Further, to solve operational issues using the v3 API will enable us to use the
backup features of the etcd Operator.
