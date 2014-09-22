package transport

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
)

func NewListener(addr string, info TLSInfo) (net.Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	if !info.Empty() {
		cfg, err := info.ServerConfig()
		if err != nil {
			return nil, err
		}

		l = tls.NewListener(l, cfg)
	}

	return l, nil
}

type TLSInfo struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

func (info TLSInfo) Empty() bool {
	return info.CertFile == "" && info.KeyFile == ""
}

// Generates a tls.Config object for a server from the given files.
func (info TLSInfo) ServerConfig() (*tls.Config, error) {
	// Both the key and cert must be present.
	if info.KeyFile == "" || info.CertFile == "" {
		return nil, fmt.Errorf("KeyFile and CertFile must both be present[key: %v, cert: %v]", info.KeyFile, info.CertFile)
	}

	var cfg tls.Config

	tlsCert, err := tls.LoadX509KeyPair(info.CertFile, info.KeyFile)
	if err != nil {
		return nil, err
	}

	cfg.Certificates = []tls.Certificate{tlsCert}

	if info.CAFile != "" {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		cp, err := newCertPool(info.CAFile)
		if err != nil {
			return nil, err
		}

		cfg.RootCAs = cp
		cfg.ClientCAs = cp
	} else {
		cfg.ClientAuth = tls.NoClientCert
	}

	return &cfg, nil
}

// newCertPool creates x509 certPool with provided CA file
func newCertPool(CAFile string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()
	pemByte, err := ioutil.ReadFile(CAFile)
	if err != nil {
		return nil, err
	}

	for {
		var block *pem.Block
		block, pemByte = pem.Decode(pemByte)
		if block == nil {
			return certPool, nil
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certPool.AddCert(cert)
	}
}
