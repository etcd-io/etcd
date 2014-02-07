package server

import (
	"strings"
)

// usage defines the message shown when a help flag is passed to etcd.
var usage = `
etcd

Usage:
  etcd -name <name>
  etcd -name <name> [-data-dir=<path>]
  etcd -h | -help
  etcd -version

Options:
  -h -help          Show this screen.
  --version         Show version.
  -f -force         Force a new configuration to be used.
  -config=<path>    Path to configuration file.
  -name=<name>      Name of this node in the etcd cluster.
  -data-dir=<path>  Path to the data directory.
  -cors=<origins>   Comma-separated list of CORS origins.
  -v                Enabled verbose logging.
  -vv               Enabled very verbose logging.

Cluster Configuration Options:
  -discovery=<url>                Discovery service used to find a peer list.
  -peers-file=<path>              Path to a file containing the peer list.
  -peers=<host:port>,<host:port>  Comma-separated list of peers. The members
                                  should match the peer's '-peer-addr' flag.

Client Communication Options:
  -addr=<host:port>         The public host:port used for client communication.
  -bind-addr=<host[:port]>  The listening host:port used for client communication.
  -ca-file=<path>           Path to the client CA file.
  -cert-file=<path>         Path to the client cert file.
  -key-file=<path>          Path to the client key file.

Peer Communication Options:
  -peer-addr=<host:port>  The public host:port used for peer communication.
  -peer-bind-addr=<host[:port]>  The listening host:port used for peer communication.
  -peer-ca-file=<path>    Path to the peer CA file.
  -peer-cert-file=<path>  Path to the peer cert file.
  -peer-key-file=<path>   Path to the peer key file.
  -peer-heartbeat-timeout=<time>
                          Time (in milliseconds) for a heartbeat to timeout.
  -peer-election-timeout=<time>
                          Time (in milliseconds) for an election to timeout.

Other Options:
  -max-result-buffer   Max size of the result buffer.
  -max-retry-attempts  Number of times a node will try to join a cluster.
  -retry-interval      Seconds to wait between cluster join retry attempts.
  -max-cluster-size    Maximum number of nodes in the cluster.
  -snapshot=false      Disable log snapshots
  -snapshot-count      Number of transactions before issuing a snapshot.
`

// Usage returns the usage message for etcd.
func Usage() string {
	return strings.TrimSpace(usage)
}
