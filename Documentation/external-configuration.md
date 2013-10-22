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

* `-n` - The node name. Defaults to `default-name`.

### Optional

* `-c` - The advertised public hostname:port for client communication. Defaults to `127.0.0.1:4001`.
* `-cl` - The listening hostname for client communication. Defaults to advertised ip.
* `-C` - A comma separated list of machines in the cluster (i.e `"203.0.113.101:7001,203.0.113.102:7001"`).
* `-CF` - The file path containing a comma separated list of machines in the cluster.
* `-clientCAFile` - The path of the client CAFile. Enables client cert authentication when present.
* `-clientCert` - The cert file of the client.
* `-clientKey` - The key file of the client.
* `-configfile` - The path of the etcd config file. Defaults to `/etc/etcd/etcd.toml`.
* `-cors` - A comma separated white list of origins for cross-origin resource sharing.
* `-cpuprofile` - The path to a file to output cpu profile data. Enables cpu profiling when present.
* `-d` - The directory to store log and snapshot. Defaults to the current working directory.
* `-m` - The max size of result buffer. Defaults to `1024`.
* `-maxsize` - The max size of the cluster. Defaults to `9`.
* `-r` - The max retry attempts when trying to join a cluster. Defaults to `3`.
* `-s` - The advertised public hostname:port for server communication. Defaults to `127.0.0.1:7001`.
* `-sl` - The listening hostname for server communication. Defaults to advertised ip.
* `-serverCAFile` - The path of the CAFile. Enables client/peer cert authentication when present.
* `-serverCert` - The cert file of the server.
* `-serverKey` - The key file of the server.
* `-snapshot` - Open or close snapshot. Defaults to `false`.
* `-v` - Enable verbose logging. Defaults to `false`.
* `-vv` - Enable very verbose logging. Defaults to `false`.
* `-version` - Print the version and exit.
* `-w` - The hostname:port of web interface.

## Configuration File

The etcd configuration file is written in [TOML](https://github.com/mojombo/toml)
and read from `/etc/etcd/etcd.toml` by default.

```TOML
[etcd]
  advertised_url = "127.0.0.1:4001"
  ca_file = ""
  cert_file = ""
  cors = []
  cpu_profile_file = ""
  datadir = "."
  key_file = ""
  listen_host = "127.0.0.1:4001"
  machines = []
  machines_file = ""
  max_cluster_size = 9
  max_result_buffer = 1024
  max_retry_attempts = 3
  name = "default-name"
  snapshot = false
  verbose = false
  very_verbose = false
  web_url = ""

[raft]
  advertised_url = "127.0.0.1:7001"
  ca_file = ""
  cert_file = ""
  key_file = ""
  listen_host = "127.0.0.1:7001"
```

## Environment Variables

 * `ETCD_ADVERTISED_URL`
 * `ETCD_CA_FILE`
 * `ETCD_CERT_FILE`
 * `ETCD_CORS`
 * `ETCD_CONFIG_FILE`
 * `ETCD_CPU_PROFILE_FILE`
 * `ETCD_DATADIR`
 * `ETCD_KEY_FILE`
 * `ETCD_LISTEN_HOST`
 * `ETCD_MACHINES`
 * `ETCD_MACHINES_FILE`
 * `ETCD_MAX_RETRY_ATTEMPTS`
 * `ETCD_MAX_CLUSTER_SIZE`
 * `ETCD_MAX_RESULTS_BUFFER`
 * `ETCD_NAME`
 * `ETCD_SNAPSHOT`
 * `ETCD_VERBOSE`
 * `ETCD_VERY_VERBOSE`
 * `ETCD_WEB_URL`
 * `ETCD_RAFT_ADVERTISED_URL`
 * `ETCD_RAFT_CA_FILE`
 * `ETCD_RAFT_CERT_FILE`
 * `ETCD_RAFT_KEY_FILE`
 * `ETCD_RAFT_LISTEN_HOST`

