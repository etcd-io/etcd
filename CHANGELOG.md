
## [v3.0.4](https://github.com/coreos/etcd/releases/tag/v3.0.4)

This is primarily a bug fix release, backward-compatible with all previous v3.0.0+ releases.

##### Bug fixes

- [GH5956](https://github.com/coreos/etcd/pull/5956): *: fix issue found in fast lease renew
- [GH5969](https://github.com/coreos/etcd/pull/5969): *: fix 'gogo/protobuf' compatibility issue (Fix for [GH5942](https://github.com/coreos/etcd/pull/5942))
- [GH5995](https://github.com/coreos/etcd/pull/5995): rpctypes, clientv3: retry RPC on EtcdStopped
- [GH6006](https://github.com/coreos/etcd/pull/6006): etcdmain: correctly check return values from SdNotify()
- [GH6009](https://github.com/coreos/etcd/pull/6009): v3rpc: don't elide next progress notification on progress notification
- [GH6032](https://github.com/coreos/etcd/pull/6032): fix a few issues in grpc gateway
- [GH6045](https://github.com/coreos/etcd/pull/6045): etcdserver, api, membership: don't race on setting version

##### Security changes

- [GH5991](https://github.com/coreos/etcd/pull/5991): etcdserver/api/v2http: client certificate auth via common name
- Compiled with [Go 1.6.3](https://groups.google.com/forum/#!topic/golang-nuts/kfzBGiAcxME)

##### Other changes

- [GH5951](https://github.com/coreos/etcd/pull/5951): *: update grpc-gateway and its import paths
- [GH5973](https://github.com/coreos/etcd/pull/5973): fileutil: rework purge tests so they don't poll
- [GH5976](https://github.com/coreos/etcd/pull/5976): integration: drain keepalives in TestLeaseKeepAliveCloseAfterDisconnectRevoke
- [GH5990](https://github.com/coreos/etcd/pull/5990): clientv3/integration: fix race in TestWatchCompactRevision
- [GH5999](https://github.com/coreos/etcd/pull/5999): Add support for formating output of ls command in json or extended format
- [GH6011](https://github.com/coreos/etcd/pull/6011): e2e: use a single member cluster in TestCtlV3Migrate
- [GH6047](https://github.com/coreos/etcd/pull/6047): Documentation: fix links in upgrades





## [v3.0.3](https://github.com/coreos/etcd/releases/tag/v3.0.3)

This is primarily a bug fix release, backward-compatible with all previous v3.0.0+ releases. Note that we reverted Dockerfile change to use `CMD`, because `ENTRYPOINT` does not allow to run other binaries, such as `etcdctl` ([GH5937](https://github.com/coreos/etcd/pull/5937)).

##### Bug fixes

- [GH5918](https://github.com/coreos/etcd/pull/5918): etcdmain: only get initial cluster setting if the member is not initialized
- [GH5921](https://github.com/coreos/etcd/pull/5921): raft: do not change RecentActive when resetState for progress
- [GH5948](https://github.com/coreos/etcd/pull/5948): vendor: update grpc (Fix [GH5871](https://github.com/coreos/etcd/issues/5871))

##### Other changes

- [GH5885](https://github.com/coreos/etcd/pull/5885): etcdserver: fix TestSnap
- [GH5906](https://github.com/coreos/etcd/pull/5906): e2e: add basic upgrade tests
- [GH5916](https://github.com/coreos/etcd/pull/5916): etcdctl: only takes 127.0.0.1:2379 as default endpoint





## [v3.0.2](https://github.com/coreos/etcd/releases/tag/v3.0.2)

This is primarily a bug fix release, backward-compatible with all previous v3.0.0+ releases. Note that official etcd Docker image now uses `ENTRYPOINT` instead of `CMD`, so users don't have to specify the etcd binary path ([GH5876](https://github.com/coreos/etcd/pull/5876)).

##### Bug fixes

- [GH5855](https://github.com/coreos/etcd/pull/5855): wal: release wal locks before renaming directory on init
- [GH5861](https://github.com/coreos/etcd/pull/5861): v3rpc: do not panic on user error for watch
- [GH5862](https://github.com/coreos/etcd/pull/5862): etcdserver: commit before sending snapshot
- [GH5883](https://github.com/coreos/etcd/pull/5883): clientv3: fix sync base
- [GH5888](https://github.com/coreos/etcd/pull/5888): client: make set/delete one shot operations
- [GH5897](https://github.com/coreos/etcd/pull/5897): v3rpc: lock progress and prevKV map correctly

##### Other changes

- [GH5848](https://github.com/coreos/etcd/pull/5848): etcdserver/api: print only major.minor version API





## [v3.0.1](https://github.com/coreos/etcd/releases/tag/v3.0.1)

This is primarily a bug fix release, backward-compatible with v3.0.0.

##### Bug fixes

- [GH5841](https://github.com/coreos/etcd/pull/5841): etcdserver: exit on missing backend only if semver is >= 3.0.0

##### Other changes

- [GH5843](https://github.com/coreos/etcd/pull/5843): Documentation: fix typo in api_grpc_gateway.md
- [GH5844](https://github.com/coreos/etcd/pull/5844): *: test, docs with go1.6+
