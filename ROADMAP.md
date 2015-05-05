#  etcd roadmap

**work in progress**

This document defines a high level roadmap for etcd development.

The dates below should not be considered authoritative, but rather indicative of the projected timeline of the project. The [milestones defined in GitHub](https://github.com/coreos/etcd/milestones) represent the most up-to-date and issue-for-issue plans.

etcd 2.0 is our current stable branch. The roadmap below outlines new features that will be added to etcd, and while subject to change, define what future stable will look like.

### etcd 2.1
- Security
  - Unix-style permission scheme (roles, users)./
  - Protect keys based on prefix matching
- Improved commit latency (sub-milliseconds on light load)
- Support rolling upgrades of etcd

### etcd 2.2
- Improve client libraries
  - Go
  - Python
  - Java
- etcdctl
- Stats and Metrics
  - Standardize endpoint for gathering metrics
  - Health and status endpoints
- Add benchmarks for our future targets

### etcd 3.0
- v3 API ([see also the issue tag](https://github.com/coreos/etcd/issues?utf8=%E2%9C%93&q=label%3Av3api))
  - Transactions
  - Leases
  - Binary protocol
  - Support a large number of watchers
-  Better disk backend
  - Improved write throughput
  - Support larger datasets and histories
