## Discovery Rule

Peer discovery uses the following sources in this order: `-discovery`, `-peers`, log data in `-data-dir`.

If none of these is set, it will start a new cluster by itself. If any of them is set, it will make
best efforts to find cluster, and panic if none is reachable.

If a discover URL is provided and the discovery process succeeds then it will find peers specified by the discover URL only.
This is because we assume that it has been registered in discover URL and
should not join other clusters.

If a discover URL is provided but the discovery process fails then we will prevent the node from forming
a new cluster. We assume the user doesn't want to start a brand new cluster without noticing discover URL.

## Logical Workflow

Start an etcd machine:

```
If discovery url is given:
    Do discovery
    If Success:
        Join to the cluster discovered
		return

If peer list is given:
    Try to join as follower via peer list
    If Success: return

If log data is given:
    Try to join as follower via peers in previous cluster
    If Success: return

If log data is given:
    Restart the previous cluster which is down
    return

If discovery url is given:
    Panic

If peer list is given:
    Panic

Start as the leader of a new cluster
```
