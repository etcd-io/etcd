#  etcd roadmap

**work in progress**

This document defines a high level roadmap for etcd development.

The dates below should not be considered authoritative, but rather indicative of the projected timeline of the project. The [milestones defined in GitHub](https://github.com/coreos/etcd/milestones) represent the most up-to-date and issue-for-issue plans.

etcd 3.0 is our current stable branch. The roadmap below outlines new features that will be added to etcd, and while subject to change, define what future stable will look like.

### etcd 3.1 (2016-Oct)
- Stable L4 gateway
- Experimental support for scalable proxy
- Automatic leadership transfer for the rolling upgrade
- V3 API improvements
  - Get previous key-value pair
  - Get only keys (ignore values)
  - Get only key count

### etcd 3.2 (2017-Apr)
- Stable scalable proxy
- Proxy-as-client interface passthrough
- Lock service
- Namespacing proxy
- JWT token based authentication
- Read-modify-write V3 Put
- Improved watch performance

### etcd 3.3 (?)
- TBD

