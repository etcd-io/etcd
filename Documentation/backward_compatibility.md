### Backward Compatibility

The main goal of etcd 2.0 release is to improve cluster safety around bootstrapping and dynamic reconfiguration. To do this, we deprecated the old error-prone APIs and provide a new set of APIs.

The other main focus of this release was a more reliable Raft implementation, but as this change is internal it should not have any notable effects to users.

#### Command Line Flags Changes

The major flag changes are to mostly related to bootstrapping. The `initial-*` flags provide an improved way to specify the required criteria to start the cluster. The advertised URLs now support a list of values instead of a single value, which allows etcd users to gracefully migrate to the new set of IANA-assigned ports (2379/client and 2380/peers) while maintaining backward compatibility with the old ports.

 - `-addr` is replaced by `-advertise-client-urls`.
 - `-bind-addr` is replaced by `-listen-client-urls`.
 - `-peer-addr` is replaced by `-initial-advertise-peer-urls`.
 - `-peer-bind-addr` is replaced by `-listen-peer-urls`.
 - `-peers` is replaced by `-initial-cluster`.
 - `-peers-file` is replaced by `-initial-cluster`.
 - `-peer-heartbeat-interval` is replaced by `-heartbeat-interval`.
 - `-peer-election-timeout` is replaced by `-election-timeout`.

The documentation of new command line flags can be found at
https://github.com/coreos/etcd/blob/master/Documentation/configuration.md.

#### Data Dir
- Default data dir location has changed from {$hostname}.etcd to {name}.etcd.

- The disk format within the data dir has changed. etcd 2.0 should be able to auto upgrade the old data format. Instructions on doing so manually are in the [migration tool doc][migrationtooldoc].

[migrationtooldoc]: https://github.com/coreos/etcd/blob/master/Documentation/0_4_migration_tool.md

#### Standby

etcd 0.4’s standby mode has been deprecated. [Proxy mode][proxymode] is introduced to solve a subset of problems standby was solving.

Standby mode was intended for large clusters that had a subset of the members acting in the consensus process. Overall this process was too magical and allowed for operators to back themselves into a corner.

Proxy mode in 2.0 will provide similar functionality, and with improved control over which machines act as proxies due to the operator specifically configuring them. Proxies also support read only or read/write modes for increased security and durability.

[proxymode]: https://github.com/coreos/etcd/blob/master/Documentation/proxy.md

#### Discovery Service

A size key needs to be provided inside a [discovery token][discoverytoken].
[discoverytoken]: https://github.com/coreos/etcd/blob/master/Documentation/clustering.md#custom-etcd-discovery-service

#### HTTP Admin API

`v2/admin` on peer url and `v2/keys/_etcd` are unified under the new [v2/member API][memberapi] to better explain which machines are part of an etcd cluster, and to simplify the keyspace for all your use cases.

[memberapi]: https://github.com/coreos/etcd/blob/master/Documentation/other_apis.md

#### HTTP Key Value API
- The follower can now transparently proxy write equests to the leader. Clients will no longer see 307 redirections to the leader from etcd.

- Expiration time is in UTC instead of local time.

