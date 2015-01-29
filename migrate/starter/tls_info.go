// Copyright 2015 CoreOS, Inc.
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

package starter

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
)

// TLSInfo holds the SSL certificates paths.
type TLSInfo struct {
	CertFile string `json:"CertFile"`
	KeyFile  string `json:"KeyFile"`
	CAFile   string `json:"CAFile"`
}

func (info TLSInfo) Scheme() string {
	if info.KeyFile != "" && info.CertFile != "" {
		return "https"
	} else {
		return "http"
	}
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

// Generates a tls.Config object for a client from the given files.
func (info TLSInfo) ClientConfig() (*tls.Config, error) {
	var cfg tls.Config

	if info.KeyFile == "" || info.CertFile == "" {
		return &cfg, nil
	}

	tlsCert, err := tls.LoadX509KeyPair(info.CertFile, info.KeyFile)
	if err != nil {
		return nil, err
	}

	cfg.Certificates = []tls.Certificate{tlsCert}

	if info.CAFile != "" {
		cp, err := newCertPool(info.CAFile)
		if err != nil {
			return nil, err
		}

		cfg.RootCAs = cp
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
