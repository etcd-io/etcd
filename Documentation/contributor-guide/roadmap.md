# roadmap

etcd uses GitHub milestones to track all tasks in each major or minor release. The roadmap.md file only records the
most important tasks for each release. The list is based on current maintainers capacity that may shift over time.
Proposed milestones is what we think we can deliver with people we have. If we have more support on the important
stuff, we could pick up more items from backlog. Note that etcd will continue to mainly focus on technical debt over
the next few major or minor releases.

Each item has an assigned priority:
- P0 - Critical for the current milestone, and blocks the release.
- P1 - Important for the current milestone, and critical for the next milestone.
- P2 - Nice to have, can be always skipped and should not block anything.

## v3.6.0

For a full list of tasks in v3.6.0, please see [milestone etcd-v3.6](https://github.com/etcd-io/etcd/milestone/38).

| Title                                                                                                              | Priority | Note                                                                                                         |
|--------------------------------------------------------------------------------------------------------------------|----------|--------------------------------------------------------------------------------------------------------------|
| [Support downgrade](https://github.com/etcd-io/etcd/issues/11716)                                                  | P0       | etcd will support downgrade starting from 3.6.0. But it will also support offline downgrade from 3.5 to 3.4. |
| [StoreV2 deprecation](https://github.com/etcd-io/etcd/issues/12913)                                                | P0       | This task will be covered in both 3.6 and 3.7.                                                               |
| [Release raft 3.6.0](https://github.com/etcd-io/raft/issues/89)                                                    | P0       | etcd 3.6.0 will depends on raft 3.6.0                                                                        |
| [Release bbolt 1.4.0](https://github.com/etcd-io/bbolt/issues/553)                                                 | P0       | etcd 3.6.0 will depends on bbolt 1.4.0                                                                       |
| [Support /livez and /readyz endpoints](https://github.com/etcd-io/etcd/issues/16007)                               | P1       | It provides clearer APIs, and can also workaround the stalled writes issue                                   |
| [Bump gRPC](https://github.com/etcd-io/etcd/issues/16290)                                                          | P1       | It isn't guaranteed to be resolved in 3.6, and might be postponed to 3.7 depending on the effort and risk.   |
| [Deprecate grpc-gateway or bump it](https://github.com/etcd-io/etcd/issues/14499)                                  | P1       | It isn't guaranteed to be resolved in 3.6, and might be postponed to 3.7 depending on the effort and risk.   |
| [bbolt: Add logger into bbolt](https://github.com/etcd-io/bbolt/issues/509)                                        | P1       | It's important to diagnose bbolt issues                                                                      |
| [bbolt: Add surgery commands](https://github.com/etcd-io/bbolt/issues/370)                                         | P1       | Surgery commands are important for fixing corrupted db files                                                 |
| [Evaluate and (Gradulate or deprecate/remove) experimental features](https://github.com/etcd-io/etcd/issues/16292) | P2       | This task will be covered in both 3.6 and 3.7.                                                               |

## v3.7.0

For a full list of tasks in v3.7.0, please see [milestone etcd-v3.7](https://github.com/etcd-io/etcd/milestone/39).

| Title                                                                                                             | Priority | Note                                                                              |
|-------------------------------------------------------------------------------------------------------------------|----------|-----------------------------------------------------------------------------------|
| [StoreV2 deprecation](https://github.com/etcd-io/etcd/issues/12913)                                               | P0       | Finish the remaining tasks 3.7.                                                   |
| [Refactor lease: Lease might be revoked by mistake by old leader](https://github.com/etcd-io/etcd/issues/15247)   | P1       | to be investigated & discussed                                                    |
| [Integrate raft's new feature (async write) into etcd](https://github.com/etcd-io/etcd/issues/16291)              | P1       | It should can improve the performance                                             |
| [bbolt: Support customizing the bbolt rebalance threshold](https://github.com/etcd-io/bbolt/issues/422)           | P2       | It may get rid of etcd's defragmentation. Both bbolt and etcd need to be changed. |
| [Evaluate and (graduate or deprecate/remove) experimental features](https://github.com/etcd-io/etcd/issues/16292) | P2       | Finish the remaining tasks 3.7.                                                   |

## Backlog (future releases)

| Title                                                                                                    | Priority | Note |
|----------------------------------------------------------------------------------------------------------|----------|------|
| [Remove the dependency on grpc-go's experimental API](https://github.com/etcd-io/etcd/issues/15145)      |          |      |
| [Protobuf: cleanup both golang/protobuf and gogo/protobuf](https://github.com/etcd-io/etcd/issues/14533) |          |      |
| [Proposals should include a merkle root](https://github.com/etcd-io/etcd/issues/13839)                   |          |      |
| [Add Distributed Tracing using OpenTelemetry](https://github.com/etcd-io/etcd/issues/12460)              |          |      |
| [Support CA rotation](https://github.com/etcd-io/etcd/issues/11555)                                      |          |      |
| [bbolt: Migrate all commands to cobra style commands](https://github.com/etcd-io/bbolt/issues/472)       |          |      |
| [raft: enhance the configuration change validation](https://github.com/etcd-io/raft/issues/80)           |          |      |
