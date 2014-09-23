package transport

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
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

func NewTransport(info TLSInfo) (*http.Transport, error) {
	t := &http.Transport{
		// timeouts taken from http.DefaultTransport
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if !info.Empty() {
		tlsCfg, err := info.ClientConfig()
		if err != nil {
			return nil, err
		}
		t.TLSClientConfig = tlsCfg
	}

	return t, nil
}

type TLSInfo struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

func (info TLSInfo) Empty() bool {
	return info.CertFile == "" && info.KeyFile == ""
}

func (info TLSInfo) baseConfig() (*tls.Config, error) {
	if info.KeyFile == "" || info.CertFile == "" {
		return nil, fmt.Errorf("KeyFile and CertFile must both be present[key: %v, cert: %v]", info.KeyFile, info.CertFile)
	}

	tlsCert, err := tls.LoadX509KeyPair(info.CertFile, info.KeyFile)
	if err != nil {
		return nil, err
	}

	var cfg tls.Config
	cfg.Certificates = []tls.Certificate{tlsCert}
	return &cfg, nil
}

// ServerConfig generates a tls.Config object for use by an HTTP server
func (info TLSInfo) ServerConfig() (*tls.Config, error) {
	cfg, err := info.baseConfig()
	if err != nil {
		return nil, err
	}

	if info.CAFile != "" {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		cp, err := newCertPool(info.CAFile)
		if err != nil {
			return nil, err
		}
		cfg.ClientCAs = cp
	} else {
		cfg.ClientAuth = tls.NoClientCert
	}

	return cfg, nil
}

// ClientConfig generates a tls.Config object for use by an HTTP client
func (info TLSInfo) ClientConfig() (*tls.Config, error) {
	cfg, err := info.baseConfig()
	if err != nil {
		return nil, err
	}

	if info.CAFile != "" {
		cp, err := newCertPool(info.CAFile)
		if err != nil {
			return nil, err
		}

		cfg.RootCAs = cp
	}

	return cfg, nil
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
