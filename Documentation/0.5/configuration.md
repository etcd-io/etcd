## Configuration Flags

etcd is configurable through command-line flags and environment variables. Options set on the command line take precedence over those from the environment.

The format of environment variable for flag `-my-flag` is `ENV_MY_FLAG`. It applies to all  flags.

To start etcd automatically using custom settings at startup in Linux, using a [systemd][systemd-intro] unit is highly recommended.

[systemd-intro]: http://freedesktop.org/wiki/Software/systemd/

### Member Flags

##### -name
+ Human-readable name for this member.
+ default: "default"

##### -data-dir
+ Path to the data directory.
+ default: "${name}.etcd"

##### -snapshot-count
+ Number of committed transactions to trigger a snapshot to disk.
+ default: "10000"

##### -listen-peer-urls
+ List of URLs to listen on for peer traffic.
+ default: "http://localhost:2380,http://localhost:7001"

##### -listen-client-urls
+ List of URLs to listen on for client traffic.
+ default: "http://localhost:2379,http://localhost:4001"

##### -max-snapshots
+ Maximum number of snapshot files to retain (0 is unlimited)
+ default: 5

##### -max-wals
+ Maximum number of wal files to retain (0 is unlimited)
+ default: 5

##### -cors
+ Comma-separated white list of origins for CORS (cross-origin resource sharing).
+ default: none

### Clustering Flags

`-initial` prefix flags are used in bootstrapping ([static bootstrap][build-cluster], [discovery-service bootstrap][discovery] or [runtime reconfiguration][reconfig]) a new member, and ignored when restarting an existing member.

`-discovery` prefix flags need to be set when using [discovery service][discovery].

##### -initial-advertise-peer-urls

+ List of this member's peer URLs to advertise to the rest of the cluster. These addresses are used for communicating etcd data around the cluster. At least one must be routable to all cluster members.
+ default: "http://localhost:2380,http://localhost:7001"

##### -initial-cluster
+ Initial cluster configuration for bootstrapping.
+ default: "default=http://localhost:2380,default=http://localhost:7001"

##### initial-cluster-state
+ Initial cluster state ("new" or "existing").
+ default: "new"

##### initial-cluster-token
+ Initial cluster token for the etcd cluster during bootstrap.
+ default: "etcd-cluster"

##### advertise-client-urls
+ List of this member's client URLs to advertise to the rest of the cluster.
+ default: "http://localhost:2379,http://localhost:4001"

##### -discovery
+ Discovery URL used to bootstrap the cluster.
+ default: none

##### -discovery-fallback
+ Expected behavior ("exit" or "proxy") when discovery services fails.
+ default: "proxy"

##### -discovery-proxy
+ HTTP proxy to use for traffic to discovery service.
+ default: none

### Proxy Flags

`-proxy` prefix flags configures etcd to run in [proxy mode][proxy].

##### -proxy
+ Proxy mode setting ("off", "readonly" or "on").
+ default: "off"

### Security Flags

The security flags help to [build a secure etcd cluster][security].

##### -ca-file
+ Path to the client server TLS CA file.
+ default: none

##### -cert-file
+ Path to the client server TLS cert file.
+ default: none

##### -key-file
+ Path to the client server TLS key file.
+ default: none

##### -peer-ca-file
+ Path to the peer server TLS CA file.
+ default: none

##### -peer-cert-file
+ Path to the peer server TLS cert file.
+ default: none

##### -peer-key-file
+ Path to the peer server TLS key file.
+ default: none

### Unsafe Flags

Be CAUTIOUS to use unsafe flags because it will break the guarantee given by consensus protocol. For example, it may panic if other members in the cluster are still alive. Follow the instructions when using these falgs.

##### -force-new-cluster
+ Force to create a new one-member cluster. It commits configuration changes in force to remove all existing members in the cluster and add itself. It needs to be set to [restore a backup][restore].
+ default: false

### Miscellaneous Flags

##### -version
+ Print the version and exit.
+ default: false

[build-cluster]: https://github.com/coreos/etcd/blob/master/Documentation/0.5/clustering.md#static
[reconfig]: https://github.com/coreos/etcd/blob/master/Documentation/0.5/runtime-configuration.md
[discovery]: https://github.com/coreos/etcd/blob/master/Documentation/0.5/clustering.md#discovery
[proxy]: https://github.com/coreos/etcd/blob/master/Documentation/0.5/proxy.md
[security]: https://github.com/coreos/etcd/blob/master/Documentation/security.md
[restore]: https://github.com/coreos/etcd/blob/master/Documentation/0.5/admin_guide.md#restoring-a-backup
