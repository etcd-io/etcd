# Etcd Configuration

## Node Configuration

Individual node configuration options can be set in three places:

 1. Command line flags
 2. Environment variables
 3. Configuration file

Options set on the command line take precedence over all other sources.
Options set in environment variables take precedence over options set in
configuration files.

## Cluster Configuration

Cluster-wide settings are configured via the `/config` admin endpoint and additionally in the configuration file. Values contained in the configuration file will seed the cluster setting with the provided value. After the cluster is running, only the admin endpoint is used.

The full documentation is contained in the [API docs](https://github.com/coreos/etcd/blob/master/Documentation/api.md#cluster-config).

* `activeSize` - the maximum number of peers that can participate in the consensus protocol. Other peers will join as standbys.
* `removeDelay` - the minimum time in seconds that a machine has been observed to be unresponsive before it is removed from the cluster.
* `syncInterval` - the amount of time in seconds between cluster sync when it runs in standby mode.

## Command Line Flags

### Required

* `-name` - The node name. Defaults to a UUID.

### Optional

* `-addr` - The advertised public hostname:port for client communication. Defaults to `127.0.0.1:4001`.
* `-bind-addr` - The listening hostname for client communication. Defaults to advertised IP.
* `-ca-file` - The path of the client CAFile. Enables client cert authentication when present.
* `-cert-file` - The cert file of the client.
* `-cluster-active-size` - The expected number of instances participating in the consensus protocol. Only applied if the etcd instance is the first peer in the cluster.
* `-cluster-remove-delay` - The number of seconds before one node is removed from the cluster since it cannot be connected at all. Only applied if the etcd instance is the first peer in the cluster.
* `-cluster-sync-interval` - The number of seconds between synchronization for standby-mode instance with the cluster. Only applied if the etcd instance is the first peer in the cluster.
* `-config` - The path of the etcd configuration file. Defaults to `/etc/etcd/etcd.conf`.
* `-cors` - A comma separated white list of origins for cross-origin resource sharing.
* `-cpuprofile` - The path to a file to output CPU profile data. Enables CPU profiling when present.
* `-data-dir` - The directory to store log and snapshot. Defaults to the current working directory.
* `-discovery` - A URL to use for discovering the peer list. (i.e `"https://discovery.etcd.io/your-unique-key"`).
* `-graphite-host` - The Graphite endpoint to which to send metrics.
* `-http-read-timeout` - The number of seconds before an HTTP read operation is timed out.
* `-http-write-timeout` - The number of seconds before an HTTP write operation is timed out.
* `-key-file` - The key file of the client.
* `-max-result-buffer` - The max size of result buffer. Defaults to `1024`.
* `-max-retry-attempts` - The max retry attempts when trying to join a cluster. Defaults to `3`.
* `-peer-addr` - The advertised public hostname:port for server communication. Defaults to `127.0.0.1:7001`.
* `-peer-bind-addr` - The listening hostname for server communication. Defaults to advertised IP.
* `-peer-ca-file` - The path of the CAFile. Enables client/peer cert authentication when present.
* `-peer-cert-file` - The cert file of the server.
* `-peer-election-timeout` - The number of milliseconds to wait before the leader is declared unhealthy.
* `-peer-heartbeat-interval` - The number of milliseconds in between heartbeat requests
* `-peer-key-file` - The key file of the server.
* `-peers` - A comma separated list of peers in the cluster (i.e `"203.0.113.101:7001,203.0.113.102:7001"`).
* `-peers-file` - The file path containing a comma separated list of peers in the cluster.
* `-retry-interval` - Seconds to wait between cluster join retry attempts.
* `-snapshot=false` - Disable log snapshots. Defaults to `true`.
* `-v` - Enable verbose logging. Defaults to `false`.
* `-vv` - Enable very verbose logging. Defaults to `false`.
* `-version` - Print the version and exit.

## Configuration File

The etcd configuration file is written in [TOML](https://github.com/mojombo/toml)
and read from `/etc/etcd/etcd.conf` by default.

```TOML
addr = "127.0.0.1:4001"
bind_addr = "127.0.0.1:4001"
ca_file = ""
cert_file = ""
cors = []
cpu_profile_file = ""
data_dir = "."
discovery = "http://etcd.local:4001/v2/keys/_etcd/registry/examplecluster"
http_read_timeout = 10.0
http_write_timeout = 10.0
key_file = ""
peers = []
peers_file = ""
max_result_buffer = 1024
max_retry_attempts = 3
name = "default-name"
snapshot = true
verbose = false
very_verbose = false

[peer]
addr = "127.0.0.1:7001"
bind_addr = "127.0.0.1:7001"
ca_file = ""
cert_file = ""
key_file = ""

[cluster]
active_size = 9
remove_delay = 1800.0
sync_interval = 5.0
```

## Environment Variables

 * `ETCD_ADDR`
 * `ETCD_BIND_ADDR`
 * `ETCD_CA_FILE`
 * `ETCD_CERT_FILE`
 * `ETCD_CLUSTER_ACTIVE_SIZE`
 * `ETCD_CLUSTER_REMOVE_DELAY`
 * `ETCD_CLUSTER_SYNC_INTERVAL`
 * `ETCD_CORS`
 * `ETCD_DATA_DIR`
 * `ETCD_DISCOVERY`
 * `ETCD_GRAPHITE_HOST`
 * `ETCD_HTTP_READ_TIMEOUT`
 * `ETCD_HTTP_WRITE_TIMEOUT`
 * `ETCD_KEY_FILE`
 * `ETCD_MAX_RESULT_BUFFER`
 * `ETCD_MAX_RETRY_ATTEMPTS`
 * `ETCD_NAME`
 * `ETCD_PEER_ADDR`
 * `ETCD_PEER_BIND_ADDR`
 * `ETCD_PEER_CA_FILE`
 * `ETCD_PEER_CERT_FILE`
 * `ETCD_PEER_ELECTION_TIMEOUT`
 * `ETCD_PEER_HEARTBEAT_INTERVAL`
 * `ETCD_PEER_KEY_FILE`
 * `ETCD_PEERS`
 * `ETCD_PEERS_FILE`
 * `ETCD_RETRY_INTERVAL`
 * `ETCD_SNAPSHOT`
 * `ETCD_SNAPSHOTCOUNT`
 * `ETCD_TRACE`
 * `ETCD_VERBOSE`
 * `ETCD_VERY_VERBOSE`
 * `ETCD_VERY_VERY_VERBOSE`
