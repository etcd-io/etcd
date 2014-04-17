## Standbys

Adding peers in an etcd cluster adds network, CPU, and disk overhead to the leader since each one requires replication.
Peers primarily provide resiliency in the event of a leader failure but the benefit of more failover nodes decreases as the cluster size increases.
A lightweight alternative is the standby.

Standbys are a way for an etcd node to forward requests along to the cluster but the standbys are not part of the Raft cluster themselves.
This provides an easier API for local applications while reducing the overhead required by a regular peer node.
Standbys also act as standby nodes in the event that a peer node in the cluster has not recovered after a long duration.


## Configuration Parameters

Standbys require two additional configuration parameters: active size & promotion delay.
The active size specifies a target size for the number of peers in the cluster.
If there are not enough peers to meet the active size then standbys are promoted to peers until the peer count is equal to the active size.
If there are more peers than the target active size then peers are demoted to standbys.

The promotion delay specifies how long the cluster should wait before removing a dead peer and promoting a standby.
By default this is 30 minutes.
If a peer is inactive for 30 minutes then the peer is removed and a live standby is found to take its place.


## Logical Workflow

Start a etcd machine and join the cluster:

```
If peer count less than active size:
  If machine already exists as a standby:
    Remove machine from standby list
  Join as peer

If peer count greater than or equal to active size:
  Join as standby
```

Remove an existing etcd machine from the cluster:

```
If machine exists in peer list:
  Remove from peer list

If machine exists in standby list:
  Remove from standby list
```

Leader's active size monitor:

```
Loop:
  Sleep 5 seconds

  If peer count less than active size:
    If standby count greater than zero:
      Request a random standby to rejoin
    Goto Loop

  If peer count greater than active size:
    Demote randomly selected peer
    Goto Loop
```

Leader's peer activity monitor:

```
Loop:
  Sleep 5 seconds

  For each peer:
    If peer last activity time greater than promote delay:
      Demote peer
      Goto Loop
```
