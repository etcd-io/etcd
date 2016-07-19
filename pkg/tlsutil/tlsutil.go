// Copyright 2016 The etcd Authors
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

package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cloudflare/cfssl/revoke"

	etcdErr "github.com/coreos/etcd/error"
)

// NewCertPool creates x509 certPool with provided CA files.
func NewCertPool(CAFiles []string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()

	for _, CAFile := range CAFiles {
		pemByte, err := ioutil.ReadFile(CAFile)
		if err != nil {
			return nil, err
		}

		for {
			var block *pem.Block
			block, pemByte = pem.Decode(pemByte)
			if block == nil {
				break
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			certPool.AddCert(cert)
		}
	}

	return certPool, nil
}

// NewCert generates TLS cert by using the given cert,key and parse function.
func NewCert(certfile, keyfile string, parseFunc func([]byte, []byte) (tls.Certificate, error)) (*tls.Certificate, error) {
	cert, err := ioutil.ReadFile(certfile)
	if err != nil {
		return nil, err
	}

	key, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}

	if parseFunc == nil {
		parseFunc = tls.X509KeyPair
	}

	tlsCert, err := parseFunc(cert, key)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}

func revokeCheckHandler(req *http.Request, CRLpath string, revokeChecker *revoke.Revoke) error {
	if req.TLS == nil {
		return nil
	}
	for _, cert := range req.TLS.PeerCertificates {
		var revoked, ok bool
		if CRLpath != "" {
			revoked, ok = revokeChecker.VerifyCertificateByCRLPath(cert, CRLpath)
		} else {
			revoked, ok = revokeChecker.VerifyCertificate(cert)
		}
		if !ok {
			return fmt.Errorf("cert check failed (CN=%s, Serial: %s)", cert.Subject.CommonName, cert.SerialNumber.String())
		}
		if revoked {
			return fmt.Errorf("Cert is revoked (CN=%s, Serial: %s)", cert.Subject.CommonName, cert.SerialNumber.String())
		}
	}
	return nil
}

func NewRevokeHandler(handler http.Handler, CRLpath string) http.Handler {
	revokeChecker := revoke.NewRevokeChecker()
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		err := revokeCheckHandler(req, CRLpath, revokeChecker)
		if err == nil {
			handler.ServeHTTP(w, req)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		e := etcdErr.NewError(etcdErr.EcodeUnauthorized, fmt.Sprint(err), 0)
		e.WriteTo(w)
	})
}
