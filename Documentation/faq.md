## Frequently Asked Questions (FAQ)

### Configuration

#### What is the difference between advertise-urls and listen-urls?

`listen-urls` specifies the local addresses etcd server binds to for accepting incoming connections. To listen on a port for all interfaces, specify `0.0.0.0` as the listen IP address.

`advertise-urls` specifies the addresses etcd clients or other etcd members should use to contact the etcd server. The advertise addresses must be reachable from the remote machines. Do not advertise addresses like `localhost` or `0.0.0.0` for a production setup since these addresses are unreachable from remote machines.

### Operation

#### How to backup a etcd cluster?

etcdctl provides a `snapshot` command to create backups. See [backup] for more details.

[backup]: https://github.com/coreos/etcd/blob/master/Documentation/op-guide/recovery.md#snapshotting-the-keyspace