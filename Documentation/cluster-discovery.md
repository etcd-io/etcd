# Cluster Discovery

## Overview

Starting an etcd cluster can be painful since each node needs to know of another node in the cluster to get started. If you are trying to bring up a cluster all at once, say using a cloud formation, you also need to coordinate who will be the initial cluster leader. The discovery protocol helps you by providing an automated way to discover other existing peers in a cluster.

## Using the Discovery API

To use the discovery API, you must first create a token for your etcd cluster. Visit https://discovery.etcd.io/new to create a new token.

When each etcd instance is started, specify the token in the `-discovery` flag. This will allow each instance to download the list of existing peers in the cluster. If the instance is the first peer, it will start as the leader of the cluster.

You can inspect the list of peers by viewing `https://discovery.etcd.io/<token>`.

## Stale Peers

etcd will automatically clean up the address of a stale peer that is no longer part of the cluster. The TTL for this process is a week, which should be long enough to handle any extremely long outage you may encounter. There is no harm in having stale peers in the list until they are cleaned up, since an etcd instance only needs to connect to one valid peer in the cluster to join.

## Supported Platforms

etcd discovery works on any platform that can read traffic from https://discovery.etcd.io. For example, you can use the same token across cloud servers and bare metal. Be sure to configure `-peer-addr` with an address that all peers can communicate with (no private IPs!).