# Why etcd

etcd stores metadata in a consistent and fault-tolerant way. Distributed systems use etcd as a consistent key-value store for configuration management, service discovery, and coordinating distributed work. Common distributed patterns using etcd include leader election, [distributed locks][etcd-concurrency], and monitoring machine liveness.

## Use cases

- Container Linux by CoreOS: Application running on [Container Linux][container-linux] gets automatic, zero-downtime Linux kernel updates. Container Linux uses [locksmith] to coordinate updates. locksmith implements a distributed semaphore over etcd to ensure only a subset of a cluster is rebooting at any given time.
- [Kubernetes][kubernetes] stores configuration data into etcd for service discovery and cluster management; etcd's consistency is crucial for correctly scheduling and operating services. The Kubernetes API server persists cluster state into etcd. It uses etcd's watch API to monitor the cluster and roll out critical configuration changes.


## Features and system comparisons

TODO

[etcd-concurrency]: https://godoc.org/github.com/coreos/etcd/clientv3/concurrency
[container-linux]: https://coreos.com/why
[locksmith]: https://github.com/coreos/locksmith
[kubernetes]: http://kubernetes.io/docs/whatisk8s

