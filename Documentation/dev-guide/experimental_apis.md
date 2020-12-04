---
title: Experimental APIs and features
weight: 3750
description: New (and potentially unstable) APIs and features in etcd
---

For the most part, the etcd project is stable, but we are still moving fast! We believe in the release fast philosophy. We want to get early feedback on features still in development and stabilizing. Thus, there are, and will be more, experimental features and APIs. We plan to improve these features based on the early feedback from the community, or abandon them if there is little interest, in the next few releases. Please do not rely on any experimental features or APIs in production environment.

## The current experimental API/features are:

- [KV ordering](https://godoc.org/github.com/etcd-io/etcd/clientv3/ordering) wrapper. When an etcd client switches endpoints, responses to serializable reads may go backward in time if the new endpoint is lagging behind the rest of the cluster. The ordering wrapper caches the current cluster revision from response headers. If a response revision is less than the cached revision, the client selects another endpoint and reissues the read. Enable in grpcproxy with `--experimental-serializable-ordering`.
