## Configuration Flags

etcd is configurable through command-line flags and environment variables. Options set on the command line take precedence over those from the environment.

The format of environment variable for flag `-my-flag` is `ETCD_MY_FLAG`. It applies to all  flags.

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

##### -heartbeat-interval
+ Time (in milliseconds) of a heartbeat interval.
+ default: "100"

##### -election-timeout
+ Time (in milliseconds) for an election to timeout.
+ default: "1000"

##### -listen-peer-urls
+ List of URLs to listen on for peer traffic.
+ default: "http://localhost:2380,http://localhost:7001"

##### -listen-client-urls
+ List of URLs to listen on for client traffic.
+ default: "http://localhost:2379,http://localhost:4001"

##### -max-snapshots
+ Maximum number of snapshot files to retain (0 is unlimited)
+ default: 5
+ The default for users on Windows is unlimited, and manual purging down to 5 (or your preference for safety) is recommended.

##### -max-wals
+ Maximum number of wal files to retain (0 is unlimited)
+ default: 5
+ The default for users on Windows is unlimited, and manual purging down to 5 (or your preference for safety) is recommended.

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

##### -initial-cluster-state
+ Initial cluster state ("new" or "existing"). Set to `new` for all members present during initial static or DNS bootstrapping. If this option is set to `existing`, etcd will attempt to join the existing cluster. If the wrong value is set, etcd will attempt to start but fail safely.
+ default: "new"

[static bootstrap]: clustering.md#static

##### -initial-cluster-token
+ Initial cluster token for the etcd cluster during bootstrap.
+ default: "etcd-cluster"

##### -advertise-client-urls
+ List of this member's client URLs to advertise to the rest of the cluster.
+ default: "http://localhost:2379,http://localhost:4001"

##### -discovery
+ Discovery URL used to bootstrap the cluster.
+ default: none

##### -discovery-srv
+ DNS srv domain used to bootstrap the cluster.
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

##### -ca-file [DEPRECATED]
+ Path to the client server TLS CA file.
+ default: none

##### -cert-file
+ Path to the client server TLS cert file.
+ default: none

##### -key-file
+ Path to the client server TLS key file.
+ default: none

##### -client-cert-auth
+ Enable client cert authentication.
+ default: false

##### -trusted-ca-file
+ Path to the client server TLS trusted CA key file.
+ default: none

##### -peer-ca-file [DEPRECATED]
+ Path to the peer server TLS CA file.
+ default: none

##### -peer-cert-file
+ Path to the peer server TLS cert file.
+ default: none

##### -peer-key-file
+ Path to the peer server TLS key file.
+ default: none

##### -peer-client-cert-auth
+ Enable peer client cert authentication.
+ default: false

##### -peer-trusted-ca-file
+ Path to the peer server TLS trusted CA file.
+ default: none

### Logging Flags

##### -debug
+ Drop the default log level to DEBUG for all subpackages.
+ default: false (INFO for all packages)

##### -log-package-levels
+ Set individual etcd subpackages to specific log levels. An example being `etcdserver=WARNING,security=DEBUG` 
+ default: none (INFO for all packages)


### Unsafe Flags

Please be CAUTIOUS when using unsafe flags because it will break the guarantees given by the consensus protocol.
For example, it may panic if other members in the cluster are still alive.
Follow the instructions when using these flags.

##### -force-new-cluster
+ Force to create a new one-member cluster. It commits configuration changes in force to remove all existing members in the cluster and add itself. It needs to be set to [restore a backup][restore].
+ default: false

### Miscellaneous Flags

##### -version
+ Print the version and exit.
+ default: false

[build-cluster]: clustering.md#static
[reconfig]: runtime-configuration.md
[discovery]: clustering.md#discovery
[proxy]: proxy.md
[security]: security.md
[restore]: admin_guide.md#restoring-a-backup
