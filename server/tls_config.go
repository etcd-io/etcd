package server

import (
	"crypto/tls"
)

// TLSConfig holds the TLS configuration.
type TLSConfig struct {
	Scheme string     // http or https
	Server tls.Config // Used by the Raft or etcd Server transporter.
	Client tls.Config // Used by the Raft peer client.
}
