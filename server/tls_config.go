package server

import (
	"crypto/tls"
)

// TLSConfig holds the TLS configuration.
type TLSConfig struct {
	Scheme string
	Server tls.Config
	Client tls.Config
}
