# etcd go client API

This document describes using the etcd client Go package `github.com/coreos/etcd/client`.

## Endpoint Selection Mode

Programs call `client.New()` to create an etcd client instance, passing configuration through the `client.Config` parameter. `client.Config`'s `SelectionMode` member is an `EndpointSelectionMode` `enum` that specifies the policy for choosing the etcd cluster node to which requests are sent.
.

### EndpointSelectionRandom (default)

The default value of the `SelectionMode` is `EndpointSelectionRandom`. As the name implies, the client object will pick a node from the members of the cluster in a random fashion. If the cluster has three members, A, B, and C, the client picks any node from its three members as its request destination.

### EndpointSelectionPrioritizeLeader

If `SelectionMode` is set to `EndpointSelectionPrioritizeLeader`, requests are sent directly to the cluster leader. This reduces forwarding roundtrips compared to making requests to etcd followers who then forward them to the cluster leader. In the event of a leader failure, however, clients configured this way cannot prioritize among the remaining etcd followers. Therefore, when a client sets `SelectionMode` to `EndpointSelectionPrioritizeLeader`, it must use `client.AutoSync()` to maintain its knowledge of current cluster state.
