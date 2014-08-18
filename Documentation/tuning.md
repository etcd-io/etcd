## Tuning

The default settings in etcd should work well for installations on a local network where the average network latency is low.
However, when using etcd across multiple data centers or over networks with high latency you may need to tweak the heartbeat interval and election timeout settings.

The network isn't the only source of latency. Each request and response may be impacted by slow disks on both the leader and follower. Each of these timeouts represents the total time from request to successful response from the other machine.

### Time Parameters

The underlying distributed consensus protocol relies on two separate time parameters to ensure that nodes can handoff leadership if one stalls or goes offline.
The first parameter is called the *Heartbeat Interval*.
This is the frequency with which the leader will notify followers that it is still the leader.
etcd batches commands together for higher throughput so this heartbeat interval is also a delay for how long it takes for commands to be committed.
By default, etcd uses a `50ms` heartbeat interval.

The second parameter is the *Election Timeout*.
This timeout is how long a follower node will go without hearing a heartbeat before attempting to become leader itself.
By default, etcd uses a `200ms` election timeout.

Adjusting these values is a trade off.
Lowering the heartbeat interval will cause individual commands to be committed faster but it will lower the overall throughput of etcd.
If your etcd instances have low utilization then lowering the heartbeat interval can improve your command response time.

The election timeout should be set based on the heartbeat interval and your network ping time between nodes.
Election timeouts should be at least 10 times your ping time so it can account for variance in your network.
For example, if the ping time between your nodes is 10ms then you should have at least a 100ms election timeout.

You should also set your election timeout to at least 4 to 5 times your heartbeat interval to account for variance in leader replication.
For a heartbeat interval of 50ms you should set your election timeout to at least 200ms - 250ms.

You can override the default values on the command line:

```sh
# Command line arguments:
$ etcd -peer-heartbeat-interval=100 -peer-election-timeout=500

# Environment variables:
$ ETCD_PEER_HEARTBEAT_INTERVAL=100 ETCD_PEER_ELECTION_TIMEOUT=500 etcd
```

Or you can set the values within the configuration file:

```toml
[peer]
heartbeat_interval = 100
election_timeout = 500
```

The values are specified in milliseconds.


### Snapshots

etcd appends all key changes to a log file.
This log grows forever and is a complete linear history of every change made to the keys.
A complete history works well for lightly used clusters but clusters that are heavily used would carry around a large log.

To avoid having a huge log etcd makes periodic snapshots.
These snapshots provide a way for etcd to compact the log by saving the current state of the system and removing old logs.

### Snapshot Tuning

Creating snapshots can be expensive so they're only created after a given number of changes to etcd.
By default, snapshots will be made after every 10,000 changes.
If etcd's memory usage and disk usage are too high, you can lower the snapshot threshold by setting the following on the command line:

```sh
# Command line arguments:
$ etcd -snapshot-count=5000

# Environment variables:
$ ETCD_SNAPSHOT_COUNT=5000 etcd
```

Or you can change the setting in the configuration file:

```toml
snapshot_count = 5000
```

You can also disable snapshotting by adding the following to your command line:

```sh
# Command line arguments:
$ etcd -snapshot false

# Environment variables:
$ ETCD_SNAPSHOT=false etcd
```

You can also disable snapshotting within the configuration file:

```toml
snapshot = false
```
