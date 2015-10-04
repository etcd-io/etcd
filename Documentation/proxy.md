## Proxy

etcd can now run as a transparent proxy. Running etcd as a proxy allows for easy discovery of etcd within your infrastructure, since it can run on each machine as a local service. In this mode, etcd acts as a reverse proxy and forwards client requests to an active etcd cluster. The etcd proxy does not participate in the consensus replication of the etcd cluster, thus it neither increases the resilience nor decreases the write performance of the etcd cluster.

etcd currently supports two proxy modes: `readwrite` and `readonly`. The default mode is `readwrite`, which forwards both read and write requests to the etcd cluster. A `readonly` etcd proxy only forwards read requests to the etcd cluster, and returns `HTTP 501` to all write requests. 

The proxy will shuffle the list of cluster members periodically to avoid sending all connections to a single member.

The member list used by proxy consists of all client URLs advertised within the cluster, as specified in each members' `-advertise-client-urls` flag. If this flag is set incorrectly, requests sent to the proxy are forwarded to wrong addresses and then fail. Including URLs in the `-advertise-client-urls` flag that point to the proxy itself, e.g. http://localhost:2379, is even more problematic as it will cause loops, because the proxy keeps trying to forward requests to itself until its resources (memory, file descriptors) are eventually depleted. The fix for this problem is to restart etcd member with correct `-advertise-client-urls` flag. After client URLs list in proxy is recalculated, which happens every 30 seconds, requests will be forwarded correctly.

### Using an etcd proxy
To start etcd in proxy mode, you need to provide three flags: `proxy`, `listen-client-urls`, and `initial-cluster` (or `discovery`). 

To start a readwrite proxy, set `-proxy on`; To start a readonly proxy, set `-proxy readonly`.

The proxy will be listening on `listen-client-urls` and forward requests to the etcd cluster discovered from in `initial-cluster` or `discovery` url. 

#### Start an etcd proxy with a static configuration
To start a proxy that will connect to a statically defined etcd cluster, specify the `initial-cluster` flag:

```
etcd -proxy on \
-listen-client-urls http://127.0.0.1:8080 \
-initial-cluster infra0=http://10.0.1.10:2380,infra1=http://10.0.1.11:2380,infra2=http://10.0.1.12:2380
```

#### Start an etcd proxy with the discovery service
If you bootstrap an etcd cluster using the [discovery service][discovery-service], you can also start the proxy with the same `discovery`. 

To start a proxy using the discovery service, specify the `discovery` flag. The proxy will wait until the etcd cluster defined at the `discovery` url finishes bootstrapping, and then start to forward the requests. 

```
etcd -proxy on \
-listen-client-urls http://127.0.0.1:8080 \
-discovery https://discovery.etcd.io/3e86b59982e49066c5d813af1c2e2579cbf573de
```

#### Fallback to proxy mode with discovery service
If you bootstrap a etcd cluster using [discovery service][discovery-service] with more than the expected number of etcd members, the extra etcd processes will fall back to being `readwrite` proxies by default. They will forward the requests to the cluster as described above. For example, if you create a discovery url with `size=5`, and start ten etcd processes using that same discovery url, the result will be a cluster with five etcd members and five proxies. Note that this behaviour can be disabled with the `proxy-fallback` flag.

### Promote a proxy to a member of etcd cluster

A Proxy is in the part of etcd cluster that does not participant in consensus. A proxy will not promote itself to an etcd member that participants in consensus automtically in any case.

If you want to promote a proxy to an etcd member, there are four steps you need to follow:

- use etcdctl to add the proxy node as an etcd member into the existing cluster
- stop the etcd proxy process or service
- remove the existing proxy data directory
- restart the etcd process with new member configuration

#### Example

We assume you have a one member etcd cluster with one proxy. The cluster information is listed below:

|Name|Address|
|------|---------|
|infra0|10.0.1.10|
|proxy0|10.0.1.11|

This example walks you through a case that you promote one proxy to an etcd member. The cluster will become a two member cluster after finishing the four steps.

### Add a new member into the existing cluster

First, use etcdctl to add the member to the cluster, which will output the environment variables need to correctly configure the new member:

``` bash
$ etcdctl -endpoint http://10.0.1.10:2379 member add infra1 http://10.0.1.11:2380
added member 9bf1b35fc7761a23 to cluster

ETCD_NAME="infra1"
ETCD_INITIAL_CLUSTER="infra0=http://10.0.1.10:2380,infra1=http://10.0.1.11:2380"
ETCD_INITIAL_CLUSTER_STATE=existing
```

### Stop the proxy process

Stop the existing proxy so we can wipe it's state on disk and reload it with the new configuration:

``` bash
px aux | grep etcd
kill %etcd_proxy_pid%
```

or (if you are running etcd proxy as etcd service under systemd)

``` bash
sudo systemctl stop etcd
```

### Remove the existing proxy data dir

``` bash
rm -rf %data_dir%/proxy
```

### Start etcd as a new member

Finally, start the reconfigured member and make sure it joins the cluster correctly:

``` bash
$ export ETCD_NAME="infra1"
$ export ETCD_INITIAL_CLUSTER="infra0=http://10.0.1.10:2380,infra1=http://10.0.1.11:2380"
$ export ETCD_INITIAL_CLUSTER_STATE=existing
$ etcd -listen-client-urls http://10.0.1.11:2379 \
-advertise-client-urls http://10.0.1.11:2379 \
-listen-peer-urls http://10.0.1.11:2380 \
-initial-advertise-peer-urls http://10.0.1.11:2380 \
-data-dir %data_dir%
```

If you are running etcd under systemd, you should modify the service file with correct configuration and restart the service:

``` bash
sudo systemd restart etcd
```

If you see an error, you can read the [add member troubleshooting doc](runtime-configuration.md#error-cases).

[discovery-service]: clustering.md#discovery
