#  etcd roadmap

**work in progress**

This document defines a high level roadmap for etcd development.

The dates below should not be considered authoritative, but rather indicative of the projected timeline of the project. The [milestones defined in GitHub](https://github.com/coreos/etcd/milestones) represent the most up-to-date and issue-for-issue plans.

etcd 2.2 is our current stable branch. The roadmap below outlines new features that will be added to etcd, and while subject to change, define what future stable will look like.

### etcd 2.3 (October)
- improved v3 API
	- support clustered API
	- support watch API
	- use gRPC error code
	- initial performance benchmark for get/put/delete
	- initial API level testing
- better backend 
	- fully tested backend
	- benchmark performance for key operations
	- benchmark memory usage
- experimental v3 compatibility
	- store v2 snapshot into new backend
	- move snapshot logic out of raft to support new snapshot work-flow
- simple v3 client (maybe)

### etcd 3.0 (January)
- v3 API ([see also the issue tag](https://github.com/coreos/etcd/issues?utf8=%E2%9C%93&q=label%3Av3api))
  - Transactions
  - Leases
  - Binary protocol
  - Support a large number of watchers
-  Better disk backend
  - Improved write throughput
  - Support larger datasets and histories
