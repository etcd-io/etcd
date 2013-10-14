package server

import (
	"crypto/tls"
)

type TLSConfig struct {
	Scheme string
	Server tls.Config
	Client tls.Config
}
