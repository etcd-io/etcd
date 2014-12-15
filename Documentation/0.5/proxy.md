## Proxy

etcd can now run as a transparent proxy. Running etcd as a proxy allows for easily discovery of etcd within your infrastructure, since it can run on each machine as a local service. In this mode, etcd acts as a reverse proxy and forwards client requests to an active etcd cluster. The etcd proxy does not participant in the consensus replication of the etcd cluster, thus it neither increases the resilience nor decreases the write performance of the etcd cluster. 

etcd currently supports two proxy modes: `readwrite` and `readonly`. The default mode is `readwrite`, which forwards both read and write requests to the etcd cluster. A `readonly` etcd proxy only forwards read requests to the etcd cluster, and returns `HTTP 501` to all write requests. 

### Using an etcd proxy
To start etcd in proxy mode, you need to provide three flags: `proxy`, `listen-client-urls`, and `initial-cluster` (or `discovery-url`). 

To start a readwrite proxy, set `-proxy on`; To start a readonly proxy, set `-proxy readonly`.

The proxy will be listening on `listen-client-urls` and forward requests to the etcd cluster discovered from in `initial-cluster` or `discovery url`. 

#### Start an etcd proxy with a static configuration
To start a proxy that will connect to a statically defined etcd cluster, specify the `initial-cluster` flag:
```
etcd -proxy on -client-listen-urls 127.0.0.1:8080 -initial-cluster infra0=http://10.0.1.10:2380,infra1=http://10.0.1.11:2380,infra2=http://10.0.1.12:2380
```

#### Start an etcd proxy with the discovery service
If you bootstrap an etcd cluster using the [discovery service][discovery-service], you can also start the proxy with the same `discovery-url`. 

To start a proxy using the discovery service, specify the `discovery-url` flag. The proxy will wait until the etcd cluster defined at the `discovery-url` finishes bootstrapping, and then start to forward the requests. 

```
etcd -proxy on -client-listen-urls 127.0.0.1:8080  -discovery https://discovery.etcd.io/3e86b59982e49066c5d813af1c2e2579cbf573de
```

#### Fallback to proxy mode with discovery service
If you bootstrap a etcd cluster using [discovery service][discovery-service] with more than the expected number of etcd members, the extra etcd processes will fall back to being `readwrite` proxies by default. They will forward the requests to the cluster as described above. For example, if you create a discovery url with `size=5`, and start ten etcd processes using that same discovery URL, the result will be a cluster with five etcd members and five proxies. Note that this behaviour can be disabled with the `proxy-fallback` flag.

[discovery-service]: https://github.com/coreos/etcd/blob/master/Documentation/0.5/clustering.md#discovery
