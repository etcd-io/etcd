#  etcd roadmap

**work in progress**

This document defines a high level roadmap for etcd development.

The dates below should not be considered authoritative, but rather indicative of the projected timeline of the project. The [milestones defined in GitHub](https://github.com/coreos/etcd/milestones) represent the most up-to-date and issue-for-issue plans.

etcd 3.3 is our current stable branch. The roadmap below outlines new features that will be added to etcd, and while subject to change, define what future stable will look like.

### etcd 3.4 (Q2, 2018)
- Stabilization of 3.3 experimental features
- Support/document downgrade
- Snapshot restore as Go library
- Improved client balancer with new gRPC balancer interface
- Improve single-client put performance
- Improve large response handling
- Improve test coverage
- Decrease test runtime
- Migrate to `golang/dep` for dependency management

Please join our [bi-weekly etcd meeting](https://docs.google.com/document/d/1DbVXOHvd9scFsSmL2oNg4YGOHJdXqtx583DmeVWrB_M/edit) for latest discussions on roadmap. Most up-to-date roadmap can be found [here](https://docs.google.com/spreadsheets/d/1kT4xY_y1p3R8Xqp1wRDoImT7MjiepePnC14E-IACEfM/edit#gid=0).
