## Cluster Finding Process

Peer discovery uses the following sources in this order: log data in `-data-dir`, `-discovery` and `-peers`.

If log data is provided, etcd will concatenate possible peers from three sources: the log data, the `-discovery` option, and `-peers`. Then it tries to join cluster through them one by one. If all connection attempts fail (which indicates that the majority of the cluster is currently down), it will restart itself based on the log data, which helps the cluster to recover from a full outage.

Without log data, the instance is assumed to be a brand new one. If possible targets are provided by `-discovery` and `-peers`, etcd will make a best effort attempt to join them, and if none is reachable it will exit. Otherwise, if no `-discovery` or `-peers` option is provided, a new cluster will always be started.

This ensures that users can always restart the node safely with the same command (without --force), and etcd will either reconnect to the old cluster if it is still running or recover its cluster from a outage.

## Logical Workflow

Start an etcd machine:

```
If log data is given:
	Try to join via peers in previous cluster
	Try to join via peers found in discover URL
	Try to join via peers in peer list
	Restart the previous cluster which is down
	return

If discover URL is given:
	Fetch peers through discover URL
	If Success:
		Join via peers found
		return

If peer list is given:
	Join as follower via peers in peer list
	return

Start as the leader of a new cluster
```
