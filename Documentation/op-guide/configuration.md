---
title: Configuration flags
weight: 4050
description: "etcd configuration: files, flags, and environment variables"
---

etcd is configurable through a configuration file, various command-line flags, and environment variables.

A reusable configuration file is a YAML file made with name and value of one or more command-line flags described below. In order to use this file, specify the file path as a value to the `--config-file` flag or `ETCD_CONFIG_FILE` environment variable. The [sample configuration file][sample-config-file] can be used as a starting point to create a new configuration file as needed.

Options set on the command line take precedence over those from the environment. If a configuration file is provided, other command line flags and environment variables will be ignored.
For example, `etcd --config-file etcd.conf.yml.sample --data-dir /tmp` will ignore the `--data-dir` flag.

The format of environment variable for flag `--my-flag` is `ETCD_MY_FLAG`. It applies to all flags.

The [official etcd ports][iana-ports] are 2379 for client requests and 2380 for peer communication. The etcd ports can be set to accept TLS traffic, non-TLS traffic, or both TLS and non-TLS traffic.

To start etcd automatically using custom settings at startup in Linux, using a [systemd][systemd-intro] unit is highly recommended.

## Member flags

### --name
+ Human-readable name for this member.
+ default: "default"
+ env variable: ETCD_NAME
+ This value is referenced as this node's own entries listed in the `--initial-cluster` flag (e.g., `default=http://localhost:2380`). This needs to match the key used in the flag if using [static bootstrapping][build-cluster]. When using discovery, each member must have a unique name. `Hostname` or `machine-id` can be a good choice.

### --data-dir
+ Path to the data directory.
+ default: "${name}.etcd"
+ env variable: ETCD_DATA_DIR

### --wal-dir
+ Path to the dedicated wal directory. If this flag is set, etcd will write the WAL files to the walDir rather than the dataDir. This allows a dedicated disk to be used, and helps avoid io competition between logging and other IO operations.
+ default: ""
+ env variable: ETCD_WAL_DIR

### --snapshot-count
+ Number of committed transactions to trigger a snapshot to disk.
+ default: "100000"
+ env variable: ETCD_SNAPSHOT_COUNT

### --heartbeat-interval
+ Time (in milliseconds) of a heartbeat interval.
+ default: "100"
+ env variable: ETCD_HEARTBEAT_INTERVAL

### --election-timeout
+ Time (in milliseconds) for an election to timeout. See [Documentation/tuning.md][tuning] for details.
+ default: "1000"
+ env variable: ETCD_ELECTION_TIMEOUT

### --listen-peer-urls
+ List of URLs to listen on for peer traffic. This flag tells the etcd to accept incoming requests from its peers on the specified scheme://IP:port combinations. Scheme can be http or https. Alternatively, use `unix://<file-path>` or `unixs://<file-path>` for unix sockets. If 0.0.0.0 is specified as the IP, etcd listens to the given port on all interfaces. If an IP address is given as well as a port, etcd will listen on the given port and interface. Multiple URLs may be used to specify a number of addresses and ports to listen on. The etcd will respond to requests from any of the listed addresses and ports.
+ default: "http://localhost:2380"
+ env variable: ETCD_LISTEN_PEER_URLS
+ example: "http://10.0.0.1:2380"
+ invalid example: "http://example.com:2380" (domain name is invalid for binding)

### --listen-client-urls
+ List of URLs to listen on for client traffic. This flag tells the etcd to accept incoming requests from the clients on the specified scheme://IP:port combinations. Scheme can be either http or https. Alternatively, use `unix://<file-path>` or `unixs://<file-path>` for unix sockets. If 0.0.0.0 is specified as the IP, etcd listens to the given port on all interfaces. If an IP address is given as well as a port, etcd will listen on the given port and interface. Multiple URLs may be used to specify a number of addresses and ports to listen on. The etcd will respond to requests from any of the listed addresses and ports.
+ default: "http://localhost:2379"
+ env variable: ETCD_LISTEN_CLIENT_URLS
+ example: "http://10.0.0.1:2379"
+ invalid example: "http://example.com:2379" (domain name is invalid for binding)

### --max-snapshots
+ Maximum number of snapshot files to retain (0 is unlimited)
+ default: 5
+ env variable: ETCD_MAX_SNAPSHOTS
+ The default for users on Windows is unlimited, and manual purging down to 5 (or some preference for safety) is recommended.

### --max-wals
+ Maximum number of wal files to retain (0 is unlimited)
+ default: 5
+ env variable: ETCD_MAX_WALS
+ The default for users on Windows is unlimited, and manual purging down to 5 (or some preference for safety) is recommended.

### --cors
+ Comma-separated white list of origins for CORS (cross-origin resource sharing).
+ default: ""
+ env variable: ETCD_CORS

### --quota-backend-bytes
+ Raise alarms when backend size exceeds the given quota (0 defaults to low space quota).
+ default: 0
+ env variable: ETCD_QUOTA_BACKEND_BYTES

### --backend-batch-limit
+ BackendBatchLimit is the maximum operations before commit the backend transaction.
+ default: 0
+ env variable: ETCD_BACKEND_BATCH_LIMIT

### --backend-bbolt-freelist-type
+ The freelist type that etcd backend(bboltdb) uses (array and map are supported types).
+ default: map
+ env variable: ETCD_BACKEND_BBOLT_FREELIST_TYPE

### --backend-batch-interval
+ BackendBatchInterval is the maximum time before commit the backend transaction.
+ default: 0
+ env variable: ETCD_BACKEND_BATCH_INTERVAL

### --max-txn-ops
+ Maximum number of operations permitted in a transaction.
+ default: 128
+ env variable: ETCD_MAX_TXN_OPS

### --max-request-bytes
+ Maximum client request size in bytes the server will accept.
+ default: 1572864
+ env variable: ETCD_MAX_REQUEST_BYTES

### --grpc-keepalive-min-time
+ Minimum duration interval that a client should wait before pinging server.
+ default: 5s
+ env variable: ETCD_GRPC_KEEPALIVE_MIN_TIME

### --grpc-keepalive-interval
+ Frequency duration of server-to-client ping to check if a connection is alive (0 to disable).
+ default: 2h
+ env variable: ETCD_GRPC_KEEPALIVE_INTERVAL

### --grpc-keepalive-timeout
+ Additional duration of wait before closing a non-responsive connection (0 to disable).
+ default: 20s
+ env variable: ETCD_GRPC_KEEPALIVE_TIMEOUT

## Clustering flags

`--initial-advertise-peer-urls`, `--initial-cluster`, `--initial-cluster-state`, and `--initial-cluster-token` flags are used in bootstrapping ([static bootstrap][build-cluster], [discovery-service bootstrap][discovery] or [runtime reconfiguration][reconfig]) a new member, and ignored when restarting an existing member.

`--discovery` prefix flags need to be set when using [discovery service][discovery].

### --initial-advertise-peer-urls

+ List of this member's peer URLs to advertise to the rest of the cluster. These addresses are used for communicating etcd data around the cluster. At least one must be routable to all cluster members. These URLs can contain domain names.
+ default: "http://localhost:2380"
+ env variable: ETCD_INITIAL_ADVERTISE_PEER_URLS
+ example: "http://example.com:2380, http://10.0.0.1:2380"

### --initial-cluster
+ Initial cluster configuration for bootstrapping.
+ default: "default=http://localhost:2380"
+ env variable: ETCD_INITIAL_CLUSTER
+ The key is the value of the `--name` flag for each node provided. The default uses `default` for the key because this is the default for the `--name` flag.

### --initial-cluster-state
+ Initial cluster state ("new" or "existing"). Set to `new` for all members present during initial static or DNS bootstrapping. If this option is set to `existing`, etcd will attempt to join the existing cluster. If the wrong value is set, etcd will attempt to start but fail safely.
+ default: "new"
+ env variable: ETCD_INITIAL_CLUSTER_STATE

[static bootstrap]: clustering.md#static

### --initial-cluster-token
+ Initial cluster token for the etcd cluster during bootstrap.
+ default: "etcd-cluster"
+ env variable: ETCD_INITIAL_CLUSTER_TOKEN

### --advertise-client-urls
+ List of this member's client URLs to advertise to the rest of the cluster. These URLs can contain domain names.
+ default: "http://localhost:2379"
+ env variable: ETCD_ADVERTISE_CLIENT_URLS
+ example: "http://example.com:2379, http://10.0.0.1:2379"
+ Be careful if advertising URLs such as http://localhost:2379 from a cluster member and are using the proxy feature of etcd. This will cause loops, because the proxy will be forwarding requests to itself until its resources (memory, file descriptors) are eventually depleted.

### --discovery
+ Discovery URL used to bootstrap the cluster.
+ default: ""
+ env variable: ETCD_DISCOVERY

### --discovery-srv
+ DNS srv domain used to bootstrap the cluster.
+ default: ""
+ env variable: ETCD_DISCOVERY_SRV

### --discovery-srv-name
+ Suffix to the DNS srv name queried when bootstrapping using DNS.
+ default: ""
+ env variable: ETCD_DISCOVERY_SRV_NAME

### --discovery-fallback
+ Expected behavior ("exit" or "proxy") when discovery services fails. "proxy" supports v2 API only.
+ default: "proxy"
+ env variable: ETCD_DISCOVERY_FALLBACK

### --discovery-proxy
+ HTTP proxy to use for traffic to discovery service.
+ default: ""
+ env variable: ETCD_DISCOVERY_PROXY

### --strict-reconfig-check
+ Reject reconfiguration requests that would cause quorum loss.
+ default: true
+ env variable: ETCD_STRICT_RECONFIG_CHECK

### --auto-compaction-retention
+ Auto compaction retention for mvcc key value store in hour. 0 means disable auto compaction.
+ default: 0
+ env variable: ETCD_AUTO_COMPACTION_RETENTION

### --auto-compaction-mode
+ Interpret 'auto-compaction-retention' one of: 'periodic', 'revision'. 'periodic' for duration based retention, defaulting to hours if no time unit is provided (e.g. '5m'). 'revision' for revision number based retention.
+ default: periodic
+ env variable: ETCD_AUTO_COMPACTION_MODE

### --enable-v2
+ Accept etcd V2 client requests
+ default: false
+ env variable: ETCD_ENABLE_V2

## Proxy flags

`--proxy` prefix flags configures etcd to run in [proxy mode][proxy]. "proxy" supports v2 API only.

### --proxy
+ Proxy mode setting ("off", "readonly" or "on").
+ default: "off"
+ env variable: ETCD_PROXY

### --proxy-failure-wait
+ Time (in milliseconds) an endpoint will be held in a failed state before being reconsidered for proxied requests.
+ default: 5000
+ env variable: ETCD_PROXY_FAILURE_WAIT

### --proxy-refresh-interval
+ Time (in milliseconds) of the endpoints refresh interval.
+ default: 30000
+ env variable: ETCD_PROXY_REFRESH_INTERVAL

### --proxy-dial-timeout
+ Time (in milliseconds) for a dial to timeout or 0 to disable the timeout
+ default: 1000
+ env variable: ETCD_PROXY_DIAL_TIMEOUT

### --proxy-write-timeout
+ Time (in milliseconds) for a write to timeout or 0 to disable the timeout.
+ default: 5000
+ env variable: ETCD_PROXY_WRITE_TIMEOUT

### --proxy-read-timeout
+ Time (in milliseconds) for a read to timeout or 0 to disable the timeout.
+ Don't change this value if using watches because use long polling requests.
+ default: 0
+ env variable: ETCD_PROXY_READ_TIMEOUT

## Security flags

The security flags help to [build a secure etcd cluster][security].

### --ca-file

**DEPRECATED**

+ Path to the client server TLS CA file. `--ca-file ca.crt` could be replaced by `--trusted-ca-file ca.crt --client-cert-auth` and etcd will perform the same.
+ default: ""
+ env variable: ETCD_CA_FILE

### --cert-file
+ Path to the client server TLS cert file.
+ default: ""
+ env variable: ETCD_CERT_FILE

### --key-file
+ Path to the client server TLS key file.
+ default: ""
+ env variable: ETCD_KEY_FILE

### --client-cert-auth
+ Enable client cert authentication.
+ default: false
+ env variable: ETCD_CLIENT_CERT_AUTH
+ CN authentication is not supported by gRPC-gateway.

### --client-crl-file
+ Path to the client certificate revocation list file.
+ default: ""
+ env variable: ETCD_CLIENT_CRL_FILE

### --client-cert-allowed-hostname
+ Allowed Allowed TLS name for client cert authentication.
+ default: ""
+ env variable: ETCD_CLIENT_CERT_ALLOWED_HOSTNAME

### --trusted-ca-file
+ Path to the client server TLS trusted CA cert file.
+ default: ""
+ env variable: ETCD_TRUSTED_CA_FILE

### --auto-tls
+ Client TLS using generated certificates
+ default: false
+ env variable: ETCD_AUTO_TLS

### --peer-ca-file

**DEPRECATED**

+ Path to the peer server TLS CA file. `--peer-ca-file ca.crt` could be replaced by `--peer-trusted-ca-file ca.crt --peer-client-cert-auth` and etcd will perform the same.
+ default: ""
+ env variable: ETCD_PEER_CA_FILE

### --peer-cert-file
+ Path to the peer server TLS cert file. This is the cert for peer-to-peer traffic, used both for server and client.
+ default: ""
+ env variable: ETCD_PEER_CERT_FILE

### --peer-key-file
+ Path to the peer server TLS key file. This is the key for peer-to-peer traffic, used both for server and client.
+ default: ""
+ env variable: ETCD_PEER_KEY_FILE

### --peer-client-cert-auth
+ Enable peer client cert authentication.
+ default: false
+ env variable: ETCD_PEER_CLIENT_CERT_AUTH

### --peer-crl-file
+ Path to the peer certificate revocation list file.
+ default: ""
+ env variable: ETCD_PEER_CRL_FILE

### --peer-trusted-ca-file
+ Path to the peer server TLS trusted CA file.
+ default: ""
+ env variable: ETCD_PEER_TRUSTED_CA_FILE

### --peer-auto-tls
+ Peer TLS using generated certificates
+ default: false
+ env variable: ETCD_PEER_AUTO_TLS

### --peer-cert-allowed-cn
+ Allowed CommonName for inter peer authentication.
+ default: ""
+ env variable: ETCD_PEER_CERT_ALLOWED_CN

### --peer-cert-allowed-hostname
+ Allowed TLS certificate name for inter peer authentication.
+ default: ""
+ env variable: ETCD_PEER_CERT_ALLOWED_HOSTNAME

### --cipher-suites
+ Comma-separated list of supported TLS cipher suites between server/client and peers.
+ default: ""
+ env variable: ETCD_CIPHER_SUITES

## Logging flags

### --logger

**Available from v3.4.**
**WARNING: `--logger=capnslog` to be deprecated in v3.5.**

+ Specify 'zap' for structured logging or 'capnslog'.
+ default: capnslog
+ env variable: ETCD_LOGGER

### --log-outputs
+ Specify 'stdout' or 'stderr' to skip journald logging even when running under systemd, or list of comma separated output targets.
+ default: default
+ env variable: ETCD_LOG_OUTPUTS
+ 'default' use 'stderr' config for v3.4 during zap logger migraion

### --log-level

**Available from v3.4.**

+ Configures log level. Only supports debug, info, warn, error, panic, or fatal.
+ default: info
+ env variable: ETCD_LOG_LEVEL
+ 'default' use 'info'.

### --debug

**WARNING: to be deprecated in v3.5.**

+ Drop the default log level to DEBUG for all subpackages.
+ default: false (INFO for all packages)
+ env variable: ETCD_DEBUG

### --log-package-levels

**WARNING: to be deprecated in v3.5.**

+ Set individual etcd subpackages to specific log levels. An example being `etcdserver=WARNING,security=DEBUG`
+ default: "" (INFO for all packages)
+ env variable: ETCD_LOG_PACKAGE_LEVELS

## Unsafe flags

Please be CAUTIOUS when using unsafe flags because it will break the guarantees given by the consensus protocol.
For example, it may panic if other members in the cluster are still alive.
Follow the instructions when using these flags.

### --force-new-cluster
+ Force to create a new one-member cluster. It commits configuration changes forcing to remove all existing members in the cluster and add itself, but is strongly discouraged. Please review the [disaster recovery][recovery] documentation for preferred v3 recovery procedures.
+ default: false
+ env variable: ETCD_FORCE_NEW_CLUSTER

## Miscellaneous flags

### --version
+ Print the version and exit.
+ default: false

### --config-file
+ Load server configuration from a file. Note that if a configuration file is provided, other command line flags and environment variables will be ignored.
+ default: ""
+ example: [sample configuration file][sample-config-file]
+ env variable: ETCD_CONFIG_FILE

## Profiling flags

### --enable-pprof
+ Enable runtime profiling data via HTTP server. Address is at client URL + "/debug/pprof/"
+ default: false
+ env variable: ETCD_ENABLE_PPROF

### --metrics
+ Set level of detail for exported metrics, specify 'extensive' to include server side grpc histogram metrics.
+ default: basic
+ env variable: ETCD_METRICS

### --listen-metrics-urls
+ List of additional URLs to listen on that will respond to both the `/metrics` and `/health` endpoints
+ default: ""
+ env variable: ETCD_LISTEN_METRICS_URLS

## Auth flags

### --auth-token
+ Specify a token type and token specific options, especially for JWT. Its format is "type,var1=val1,var2=val2,...". Possible type is 'simple' or 'jwt'. Possible variables are 'sign-method' for specifying a sign method of jwt (its possible values are 'ES256', 'ES384', 'ES512', 'HS256', 'HS384', 'HS512', 'RS256', 'RS384', 'RS512', 'PS256', 'PS384', or 'PS512'), 'pub-key' for specifying a path to a public key for verifying jwt, 'priv-key' for specifying a path to a private key for signing jwt, and 'ttl' for specifying TTL of jwt tokens.
+ For asymmetric algorithms ('RS', 'PS', 'ES'), the public key is optional, as the private key contains enough information to both sign and verify tokens.
+ Example option of JWT: '--auth-token jwt,pub-key=app.rsa.pub,priv-key=app.rsa,sign-method=RS512,ttl=10m'
+ default: "simple"
+ env variable: ETCD_AUTH_TOKEN

### --bcrypt-cost
+ Specify the cost / strength of the bcrypt algorithm for hashing auth passwords. Valid values are between 4 and 31.
+ default: 10
+ env variable: (not supported)

### --auth-token-ttl
+ Time (in seconds) of the auth-token-ttl. Support `--auth-token=simple` model only.
+ default: 300
+ env variable: (not supported)

## Experimental flags

### --experimental-corrupt-check-time
+ Duration of time between cluster corruption check passes
+ default: 0s
+ env variable: ETCD_EXPERIMENTAL_CORRUPT_CHECK_TIME

### --experimental-compaction-batch-limit
+ Sets the maximum revisions deleted in each compaction batch.
+ default: 1000
+ env variable: ETCD_EXPERIMENTAL_COMPACTION_BATCH_LIMIT

[build-cluster]: clustering.md#static
[reconfig]: runtime-configuration.md
[discovery]: clustering.md#discovery
[iana-ports]: http://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.txt
[proxy]: /docs/v2/proxy
[restore]: /docs/v2/admin_guide#restoring-a-backup
[security]: ../security
[systemd-intro]: http://freedesktop.org/wiki/Software/systemd/
[tuning]: ../tuning.md#time-parameters
[sample-config-file]: ../../etcd.conf.yml.sample
[recovery]: ../recovery#disaster-recovery

### --experimental-peer-skip-client-san-verification
+ Skip verification of SAN field in client certificate for peer connections. This can be helpful e.g. if
cluster members run in different networks behind a NAT.

  In this case make sure to use peer certificates based on
a private certificate authority using `--peer-cert-file`, `--peer-key-file`, `--peer-trusted-ca-file`
+ default: false
+ env variable: ETCD_EXPERIMENTAL_PEER_SKIP_CLIENT_SAN_VERIFICATION
