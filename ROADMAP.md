# etcd roadmap

This document defines high level goals for project.

## Milestones

* [P0] Etcd releases are qualified by rigorous robustness testing
* [P0] Etcd can reliably detect data corruption
* [P1] Experimental features are graduated or removed
* [P1] Etcd testing is high quality, easy to maintain and expand
* [P1] Etcd v2 API and storage is removed and code cleaned up
* [P1] Etcd supports zero-downtime downgrade
* [P2] Etcd can automatically recover from data corruption

Each listed milestone should have a corresponding 
[issue](https://github.com/etcd-io/etcd/issues) or
[milestone](https://github.com/etcd-io/etcd/milestones) on GitHub. 
If it doesn't please [let us know](https://github.com/etcd-io/etcd#contact).

### Priorities

* P0 - Critical for reliability of the v3.5 and v3.4 releases. Should be prioritized this over all other work and back-ported.
* P1 - Important for long term success of the project. Blocks v3.6 release.
* P2 - Stretch goals that would be nice to have for v3.6, however should not be blocking.