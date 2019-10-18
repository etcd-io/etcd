#  etcd roadmap

**work in progress**

This document defines a high level roadmap for etcd development.

The dates below should not be considered authoritative, but rather indicative of the projected timeline of the project. The [milestones defined in GitHub](https://github.com/etcd-io/etcd/milestones) represent the most up-to-date and issue-for-issue plans.

etcd 3.3 is our current stable branch. The roadmap below outlines new features that will be added to etcd, and while subject to change, define what future stable will look like.

### etcd 3.4 (2019)

- Stabilization of 3.3 experimental features
- Support/document downgrade
- Snapshot restore as Go library
- Improved client balancer with new gRPC balancer interface
- Improve single-client put performance
- Improve large response handling
- Improve test coverage
- Decrease test runtime
- Migrate to Go module for dependency management
