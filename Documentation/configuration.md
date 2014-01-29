# Etcd Configuration

Configuration options can be set in three places:

 1. Command line flags
 2. Environment variables
 3. Configuration file

Options set on the command line take precedence over all other sources.
Options set in environment variables take precedence over options set in
configuration files.

## Command Line Flags

### Required

* `-name` - The node name. Defaults to the hostname.

### Optional

* `-addr` - The advertised public hostname:port for client communication. Defaults to `127.0.0.1:4001`.
* `-discovery` - A URL to use for discovering the peer list. (i.e `"https://discovery.etcd.io/your-unique-key"`).
* `-bind-addr` - The listening hostname for client communication. Defaults to advertised ip.
* `-peers` - A comma separated list of peers in the cluster (i.e `"203.0.113.101:7001,203.0.113.102:7001"`).
* `-peers-file` - The file path containing a comma separated list of peers in the cluster.
* `-ca-file` - The path of the client CAFile. Enables client cert authentication when present.
* `-cert-file` - The cert file of the client.
* `-key-file` - The key file of the client.
* `-config` - The path of the etcd config file. Defaults to `/etc/etcd/etcd.conf`.
* `-cors` - A comma separated white list of origins for cross-origin resource sharing.
* `-cpuprofile` - The path to a file to output cpu profile data. Enables cpu profiling when present.
* `-data-dir` - The directory to store log and snapshot. Defaults to the current working directory.
* `-max-result-buffer` - The max size of result buffer. Defaults to `1024`.
* `-max-cluster-size` - The max size of the cluster. Defaults to `9`.
* `-max-retry-attempts` - The max retry attempts when trying to join a cluster. Defaults to `3`.
* `-peer-addr` - The advertised public hostname:port for server communication. Defaults to `127.0.0.1:7001`.
* `-peer-bind-addr` - The listening hostname for server communication. Defaults to advertised ip.
* `-peer-ca-file` - The path of the CAFile. Enables client/peer cert authentication when present.
* `-peer-cert-file` - The cert file of the server.
* `-peer-key-file` - The key file of the server.
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
key_file = ""
peers = []
peers_file = ""
max_cluster_size = 9
max_result_buffer = 1024
max_retry_attempts = 3
name = "default-name"
snapshot = false
verbose = false
very_verbose = false

[peer]
addr = "127.0.0.1:7001"
bind_addr = "127.0.0.1:7001"
ca_file = ""
cert_file = ""
key_file = ""
```

## Environment Variables

 * `ETCD_ADDR`
 * `ETCD_BIND_ADDR`
 * `ETCD_CA_FILE`
 * `ETCD_CERT_FILE`
 * `ETCD_CORS_ORIGINS`
 * `ETCD_CONFIG`
 * `ETCD_CPU_PROFILE_FILE`
 * `ETCD_DATA_DIR`
 * `ETCD_KEY_FILE`
 * `ETCD_PEERS`
 * `ETCD_PEERS_FILE`
 * `ETCD_MAX_CLUSTER_SIZE`
 * `ETCD_MAX_RESULT_BUFFER`
 * `ETCD_MAX_RETRY_ATTEMPTS`
 * `ETCD_NAME`
 * `ETCD_SNAPSHOT`
 * `ETCD_VERBOSE`
 * `ETCD_VERY_VERBOSE`
 * `ETCD_PEER_ADDR`
 * `ETCD_PEER_BIND_ADDR`
 * `ETCD_PEER_CA_FILE`
 * `ETCD_PEER_CERT_FILE`
 * `ETCD_PEER_KEY_FILE`
