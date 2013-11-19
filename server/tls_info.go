package server

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
)

// TLSInfo holds the SSL certificates paths.
type TLSInfo struct {
	CertFile string `json:"CertFile"`
	KeyFile  string `json:"KeyFile"`
	CAFile   string `json:"CAFile"`
}

// Generates a TLS configuration from the given files.
func (info TLSInfo) Config() (TLSConfig, error) {
	var t TLSConfig
	t.Scheme = "http"

	// If the user do not specify key file, cert file and CA file, the type will be HTTP
	if info.KeyFile == "" && info.CertFile == "" && info.CAFile == "" {
		return t, nil
	}

	// Both the key and cert must be present.
	if info.KeyFile == "" || info.CertFile == "" {
		return t, errors.New("KeyFile and CertFile must both be present")
	}

	tlsCert, err := tls.LoadX509KeyPair(info.CertFile, info.KeyFile)
	if err != nil {
		return t, err
	}

	t.Scheme = "https"
	t.Server.ClientAuth, t.Server.ClientCAs, err = newCertPool(info.CAFile)
	if err != nil {
		return t, err
	}

	// The client should trust the RootCA that the Server uses since
	// everyone is a peer in the network.
	t.Client.Certificates = []tls.Certificate{tlsCert}
	t.Client.RootCAs = t.Server.ClientCAs

	return t, nil
}

// newCertPool creates x509 certPool and corresponding Auth Type.
// If the given CAfile is valid, add the cert into the pool and verify the clients'
// certs against the cert in the pool.
// If the given CAfile is empty, do not verify the clients' cert.
// If the given CAfile is not valid, fatal.
func newCertPool(CAFile string) (tls.ClientAuthType, *x509.CertPool, error) {
	if CAFile == "" {
		return tls.NoClientCert, nil, nil
	}
	pemByte, err := ioutil.ReadFile(CAFile)
	if err != nil {
		return 0, nil, err
	}

	block, pemByte := pem.Decode(pemByte)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return 0, nil, err
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(cert)

	return tls.RequireAndVerifyClientCert, certPool, nil
}
