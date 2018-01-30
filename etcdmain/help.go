// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcdmain

import (
	"strconv"

	"github.com/coreos/etcd/embed"
)

var (
	usageline = `usage: etcd [flags]
       start an etcd server

       etcd --version
       show the version of etcd

       etcd -h | --help
       show the help information about etcd

       etcd --config-file
       path to the server configuration file

       etcd gateway
       run the stateless pass-through etcd TCP connection forwarding proxy

       etcd grpc-proxy
       run the stateless etcd v3 gRPC L7 reverse proxy
	`
	flagsline = `
member flags:

	--name 'default'
		human-readable name for this member.
	--data-dir '${name}.etcd'
		path to the data directory.
	--wal-dir ''
		path to the dedicated wal directory.
	--snapshot-count '100000'
		number of committed transactions to trigger a snapshot to disk.
	--heartbeat-interval '100'
		time (in milliseconds) of a heartbeat interval.
	--election-timeout '1000'
		time (in milliseconds) for an election to timeout. See tuning documentation for details.
	--listen-peer-urls 'http://localhost:2380'
		list of URLs to listen on for peer traffic.
	--listen-client-urls 'http://localhost:2379'
		list of URLs to listen on for client traffic.
	--max-snapshots '` + strconv.Itoa(embed.DefaultMaxSnapshots) + `'
		maximum number of snapshot files to retain (0 is unlimited).
	--max-wals '` + strconv.Itoa(embed.DefaultMaxWALs) + `'
		maximum number of wal files to retain (0 is unlimited).
	--cors ''
		comma-separated whitelist of origins for CORS (cross-origin resource sharing).
	--quota-backend-bytes '0'
		raise alarms when backend size exceeds the given quota (0 defaults to low space quota).
	--max-txn-ops '128'
		maximum number of operations permitted in a transaction.
	--max-request-bytes '1572864'
		maximum client request size in bytes the server will accept.
	--grpc-keepalive-min-time '5s'
		minimum duration interval that a client should wait before pinging server.
	--grpc-keepalive-interval '2h'
		frequency duration of server-to-client ping to check if a connection is alive (0 to disable).
	--grpc-keepalive-timeout '20s'
		additional duration of wait before closing a non-responsive connection (0 to disable).

clustering flags:

	--initial-advertise-peer-urls 'http://localhost:2380'
		list of this member's peer URLs to advertise to the rest of the cluster.
	--initial-cluster 'default=http://localhost:2380'
		initial cluster configuration for bootstrapping.
	--initial-cluster-state 'new'
		initial cluster state ('new' or 'existing').
	--initial-cluster-token 'etcd-cluster'
		initial cluster token for the etcd cluster during bootstrap.
		Specifying this can protect you from unintended cross-cluster interaction when running multiple clusters.
	--advertise-client-urls 'http://localhost:2379'
		list of this member's client URLs to advertise to the public.
		The client URLs advertised should be accessible to machines that talk to etcd cluster. etcd client libraries parse these URLs to connect to the cluster.
	--discovery ''
		discovery URL used to bootstrap the cluster.
	--discovery-fallback 'proxy'
		expected behavior ('exit' or 'proxy') when discovery services fails.
		"proxy" supports v2 API only.
	--discovery-proxy ''
		HTTP proxy to use for traffic to discovery service.
	--discovery-srv ''
		dns srv domain used to bootstrap the cluster.
	--discovery-srv-name ''
		suffix to the dns srv name queried when bootstrapping.
	--strict-reconfig-check '` + strconv.FormatBool(embed.DefaultStrictReconfigCheck) + `'
		reject reconfiguration requests that would cause quorum loss.
	--auto-compaction-retention '0'
		auto compaction retention length. 0 means disable auto compaction.
	--auto-compaction-mode 'periodic'
		interpret 'auto-compaction-retention' one of: periodic|revision. 'periodic' for duration based retention, defaulting to hours if no time unit is provided (e.g. '5m'). 'revision' for revision number based retention.
	--enable-v2 '` + strconv.FormatBool(embed.DefaultEnableV2) + `'
		Accept etcd V2 client requests.

proxy flags:
	"proxy" supports v2 API only.

	--proxy 'off'
		proxy mode setting ('off', 'readonly' or 'on').
	--proxy-failure-wait 5000
		time (in milliseconds) an endpoint will be held in a failed state.
	--proxy-refresh-interval 30000
		time (in milliseconds) of the endpoints refresh interval.
	--proxy-dial-timeout 1000
		time (in milliseconds) for a dial to timeout.
	--proxy-write-timeout 5000
		time (in milliseconds) for a write to timeout.
	--proxy-read-timeout 0
		time (in milliseconds) for a read to timeout.


security flags:

	--ca-file '' [DEPRECATED]
		path to the client server TLS CA file. '-ca-file ca.crt' could be replaced by '-trusted-ca-file ca.crt -client-cert-auth' and etcd will perform the same.
	--cert-file ''
		path to the client server TLS cert file.
	--key-file ''
		path to the client server TLS key file.
	--client-cert-auth 'false'
		enable client cert authentication.
	--client-crl-file ''
		path to the client certificate revocation list file.
	--trusted-ca-file ''
		path to the client server TLS trusted CA cert file.
	--auto-tls 'false'
		client TLS using generated certificates.
	--peer-ca-file '' [DEPRECATED]
		path to the peer server TLS CA file. '-peer-ca-file ca.crt' could be replaced by '-peer-trusted-ca-file ca.crt -peer-client-cert-auth' and etcd will perform the same.
	--peer-cert-file ''
		path to the peer server TLS cert file.
	--peer-key-file ''
		path to the peer server TLS key file.
	--peer-client-cert-auth 'false'
		enable peer client cert authentication.
	--peer-trusted-ca-file ''
		path to the peer server TLS trusted CA file.
	--peer-auto-tls 'false'
		peer TLS using self-generated certificates if --peer-key-file and --peer-cert-file are not provided.
	--peer-crl-file ''
		path to the peer certificate revocation list file.

logging flags

	--debug 'false'
		enable debug-level logging for etcd.
	--log-package-levels ''
		specify a particular log level for each etcd package (eg: 'etcdmain=CRITICAL,etcdserver=DEBUG').
	--log-output 'default'
		specify 'stdout' or 'stderr' to skip journald logging even when running under systemd.

unsafe flags:

Please be CAUTIOUS when using unsafe flags because it will break the guarantees
given by the consensus protocol.

	--force-new-cluster 'false'
		force to create a new one-member cluster.

profiling flags:
	--enable-pprof 'false'
		Enable runtime profiling data via HTTP server. Address is at client URL + "/debug/pprof/"
	--metrics 'basic'
		Set level of detail for exported metrics, specify 'extensive' to include histogram metrics.
	--listen-metrics-urls ''
		List of URLs to listen on for metrics.

auth flags:
	--auth-token 'simple'
		Specify a v3 authentication token type and its options ('simple' or 'jwt').

experimental flags:
	--experimental-initial-corrupt-check 'false'
		enable to check data corruption before serving any client/peer traffic.
	--experimental-corrupt-check-time '0s'
		duration of time between cluster corruption check passes.
	--experimental-enable-v2v3 ''
		serve v2 requests through the v3 backend under a given prefix.
`
)
