# DNS Bootstrap

DNS SRV records can be used as a discovery mechanism for bootstrapping an etcd cluster, or configuring an etcd node running in proxy mode.

### Create DNS SRV records

```
$ dig +noall +answer SRV _etcd-server._tcp.example.com
_etcd-server._tcp.example.com. 300 IN	SRV	0 0 2380 infra0.example.com.
_etcd-server._tcp.example.com. 300 IN	SRV	0 0 2380 infra1.example.com.
_etcd-server._tcp.example.com. 300 IN	SRV	0 0 2380 infra2.example.com.
```

```
$ dig +noall +answer infra0.example.com infra1.example.com infra2.example.com
infra0.example.com.	300	IN	A	10.0.1.10
infra1.example.com.	300	IN	A	10.0.1.11
infra2.example.com.	300	IN	A	10.0.1.12
```

## Bootstrap the etcd cluster using DNS

etcd cluster memebers can listen on domain names or IP address, the bootstrap process will resolve DNS A records. The following examples will set up a 3 node etcd cluster listening on domain names:

```
$ etcd -name infra0 \
-discovery-srv example.com \
-initial-advertise-peer-urls http://infra0.example.com:2380 \
-initial-cluster-token etcd-cluster-1 \
-initial-cluster-state new \
-advertise-client-urls http://infra0.example.com:2379 \
-listen-client-urls http://infra0.example.com:2379 \
-listen-peer-urls http://infra0.example.com:2380
```

```
$ etcd -name infra1 \
-discovery-srv example.com \
-initial-advertise-peer-urls http://infra1.example.com:2380 \
-initial-cluster-token etcd-cluster-1 \
-initial-cluster-state new \
-advertise-client-urls http://infra1.example.com:2379 \
-listen-client-urls http://infra1.example.com:2379 \
-listen-peer-urls http://infra1.example.com:2380
```

```
$ etcd -name infra2 \
-discovery-srv example.com \
-initial-advertise-peer-urls http://infra2.example.com:2380 \
-initial-cluster-token etcd-cluster-1 \
-initial-cluster-state new \
-advertise-client-urls http://infra2.example.com:2379 \
-listen-client-urls http://infra2.example.com:2379 \
-listen-peer-urls http://infra2.example.com:2380
```

You can also bootstrap the cluster using IP addresses instead of domain names:

```
$ etcd -name infra0 \
-discovery-srv example.com \
-initial-advertise-peer-urls http://10.0.1.10:2380 \
-initial-cluster-token etcd-cluster-1 \
-initial-cluster-state new \
-advertise-client-urls http://10.0.1.10:2379 \
-listen-client-urls http://10.0.1.10:2379 \
-listen-peer-urls http://10.0.1.10:2380
```

```
$ etcd -name infra1 \
-discovery-srv example.com \
-initial-advertise-peer-urls http://10.0.1.11:2380 \
-initial-cluster-token etcd-cluster-1 \
-initial-cluster-state new \
-advertise-client-urls http://10.0.1.11:2379 \
-listen-client-urls http://10.0.1.11:2379 \
-listen-peer-urls http://10.0.1.11:2380
```

```
$ etcd -name infra2 \
-discovery-srv example.com \
-initial-advertise-peer-urls http://10.0.1.12:2380 \
-initial-cluster-token etcd-cluster-1 \
-initial-cluster-state new \
-advertise-client-urls http://10.0.1.12:2379 \
-listen-client-urls http://10.0.1.12:2379 \
-listen-peer-urls http://10.0.1.12:2380
```

### etcd proxy configuration

DNS SRV records can also be used to configure the list of peers for an etcd server running in proxy mode:

```
$ etcd --proxy on -discovery-srv example.com
```
