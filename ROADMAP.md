#  etcd roadmap

**work in progress**

This document defines a high level roadmap for etcd development.

The dates below should not be considered authoritative, but rather indicative of the projected timeline of the project. The [milestones defined in GitHub](https://github.com/coreos/etcd/milestones) represent the most up-to-date and issue-for-issue plans.

etcd 2.2 is our current stable branch. The roadmap below outlines new features that will be added to etcd, and while subject to change, define what future stable will look like.

### etcd 2.3alpha (November)
- v3 API preview
 	- support clustered API
        - use gRPC error code
	- initial API level testing
        - transactions
- basic runtime metrics
- better backend 
	- benchmark memory usage
- experimental v3 compatibility
	- store v2 snapshot into new backend
	- move snapshot logic out of raft to support new snapshot work-flow

### etcd 2.3 (November)
- improved v3 API preview
	- initial performance benchmark for get/put/delete
	- support watch API
- improved runtime metrics
        - raft state machine
        - new backend
        - V3 API
- better backend 
	- fully tested backend
	- benchmark performance for key operations

### etcd 3.0 (January)
- v3 API ([see also the issue tag](https://github.com/coreos/etcd/issues?utf8=%E2%9C%93&q=label%3Aarea/v3api))
    - Leases
    - Binary protocol
    - Support a large number of watchers
    - Failure guarantees documented
-  Simple v3 client (golang)

### etcd 3.1 (February)
- v3 API
    - Locking
- Better disk backend
    - Improved write throughput
    - Support larger datasets and histories
- Simpler disaster recovery UX
- Integrated with Kubernetes

### etcd 3.2 (March)
- API bindings for other languages

### etcd 3.+ (future)
- Mirroring
- Horizontally scalable proxy layer
