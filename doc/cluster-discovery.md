# Cluster Discovery

## Overview

Starting an etcd cluster requires that each node knows another in the cluster. If you are trying to bring up a cluster all at once, say using a cloud formation, you also need to coordinate who will be the initial cluster leader. The discovery protocol helps you by providing an automated way to discover other existing peers in a cluster.

For more information on how etcd can locate the cluster, see the [finding the cluster][cluster-finding] documentation.

Please note - at least 3 nodes are required for [cluster availability][optimal-cluster-size].

[cluster-finding]: https://github.com/coreos/etcd/blob/master/Documentation/design/cluster-finding.md
[optimal-cluster-size]: https://github.com/coreos/etcd/blob/master/Documentation/optimal-cluster-size.md

## Using discovery.etcd.io

### Create a Discovery URL

To use the discovery API, you must first create a unique discovery URL for your etcd cluster. Visit [https://discovery.etcd.io/new](https://discovery.etcd.io/new) to create a new discovery URL.

You can inspect the list of peers by viewing `https://discovery.etcd.io/<cluster id>`.

### Start etcd With the Discovery Flag

Specify the `-discovery` flag when you start each etcd instance. The list of existing peers in the cluster will be downloaded and configured. If the instance is the first peer, it will start as the leader of the cluster.

Here's a full example:

```
URL=$(curl https://discovery.etcd.io/new)
./etcd -name instance1 -peer-addr 10.1.2.3:7001 -addr 10.1.2.3:4001 -discovery $URL
./etcd -name instance2 -peer-addr 10.1.2.4:7001 -addr 10.1.2.4:4001 -discovery $URL
./etcd -name instance3 -peer-addr 10.1.2.5:7001 -addr 10.1.2.5:4001 -discovery $URL
```

## Running Your Own Discovery Endpoint

The discovery API communicates with a separate etcd cluster to store and retrieve the list of peers. CoreOS provides [https://discovery.etcd.io](https://discovery.etcd.io) as a free service, but you can easily run your own etcd cluster for this purpose. Here's an example using an etcd cluster located at `10.10.10.10:4001`:

```
URL="http://10.10.10.10:4001/v2/keys/testcluster"
./etcd -name instance1 -peer-addr 10.1.2.3:7001 -addr 10.1.2.3:4001 -discovery $URL
./etcd -name instance2 -peer-addr 10.1.2.4:7001 -addr 10.1.2.4:4001 -discovery $URL
./etcd -name instance3 -peer-addr 10.1.2.5:7001 -addr 10.1.2.5:4001 -discovery $URL
```

If you're interested in how to discovery API works behind the scenes, read about the [Discovery Protocol](https://github.com/coreos/etcd/blob/master/Documentation/discovery-protocol.md).

## Setting Peer Addresses Correctly

The Discovery API submits the `-peer-addr` of each etcd instance to the configured Discovery endpoint. It's important to select an address that *all* peers in the cluster can communicate with. For example, if you're located in two regions of a cloud provider, configuring a private `10.x` address will not work between the two regions, and communication will not be possible between all peers.

## Stale Peers

The discovery API will automatically clean up the address of a stale peer that is no longer part of the cluster. The TTL for this process is a week, which should be long enough to handle any extremely long outage you may encounter. There is no harm in having stale peers in the list until they are cleaned up, since an etcd instance only needs to connect to one valid peer in the cluster to join.

## Lifetime of a Discovery URL

A discovery URL identifies a single etcd cluster. Do not re-use discovery URLs for new clusters.

When a machine starts with a new discovery URL the discovery URL will be activated and record the machine's metadata. If you destroy the whole cluster and attempt to bring the cluster back up with the same discovery URL it will fail. This is intentional because all of the registered machines are gone including their logs so there is nothing to recover the killed cluster.
