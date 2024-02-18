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

package transport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/tlsutil"

	"go.uber.org/zap"
)

// NewListener creates a new listner.
func NewListener(addr, scheme string, tlsinfo *TLSInfo) (l net.Listener, err error) {
	return newListener(addr, scheme, WithTLSInfo(tlsinfo))
}

// NewListenerWithOpts creates a new listener which accpets listener options.
func NewListenerWithOpts(addr, scheme string, opts ...ListenerOption) (net.Listener, error) {
	return newListener(addr, scheme, opts...)
}

func newListener(addr, scheme string, opts ...ListenerOption) (net.Listener, error) {
	if scheme == "unix" || scheme == "unixs" {
		// unix sockets via unix://laddr
		return NewUnixListener(addr)
	}

	lnOpts := newListenOpts(opts...)

	switch {
	case lnOpts.IsSocketOpts():
		// new ListenConfig with socket options.
		config, err := newListenConfig(lnOpts.socketOpts)
		if err != nil {
			return nil, err
		}
		lnOpts.ListenConfig = config
		// check for timeout
		fallthrough
	case lnOpts.IsTimeout(), lnOpts.IsSocketOpts():
		// timeout listener with socket options.
		ln, err := newKeepAliveListener(&lnOpts.ListenConfig, addr)
		if err != nil {
			return nil, err
		}
		lnOpts.Listener = &rwTimeoutListener{
			Listener:     ln,
			readTimeout:  lnOpts.readTimeout,
			writeTimeout: lnOpts.writeTimeout,
		}
	case lnOpts.IsTimeout():
		ln, err := newKeepAliveListener(nil, addr)
		if err != nil {
			return nil, err
		}
		lnOpts.Listener = &rwTimeoutListener{
			Listener:     ln,
			readTimeout:  lnOpts.readTimeout,
			writeTimeout: lnOpts.writeTimeout,
		}
	default:
		ln, err := newKeepAliveListener(nil, addr)
		if err != nil {
			return nil, err
		}
		lnOpts.Listener = ln
	}

	//  only skip if not passing TLSInfo
	if lnOpts.skipTLSInfoCheck && !lnOpts.IsTLS() {
		return lnOpts.Listener, nil
	}
	return wrapTLS(scheme, lnOpts.tlsInfo, lnOpts.Listener)
}

func newKeepAliveListener(cfg *net.ListenConfig, addr string) (ln net.Listener, err error) {
	if cfg != nil {
		ln, err = cfg.Listen(context.TODO(), "tcp", addr)
	} else {
		ln, err = net.Listen("tcp", addr)
	}
	if err != nil {
		return
	}

	return NewKeepAliveListener(ln, "tcp", nil)
}

func wrapTLS(scheme string, tlsinfo *TLSInfo, l net.Listener) (net.Listener, error) {
	if scheme != "https" && scheme != "unixs" {
		return l, nil
	}
	if tlsinfo != nil && tlsinfo.SkipClientSANVerify {
		return NewTLSListener(l, tlsinfo)
	}
	return newTLSListener(l, tlsinfo, checkSAN)
}

func newListenConfig(sopts *SocketOpts) (net.ListenConfig, error) {
	lc := net.ListenConfig{}
	if sopts != nil {
		ctls := getControls(sopts)
		if len(ctls) > 0 {
			lc.Control = ctls.Control
		}
	}
	return lc, nil
}

type TLSInfo struct {
	// CertFile is the _server_ cert, it will also be used as a _client_ certificate if ClientCertFile is empty
	CertFile string
	// KeyFile is the key for the CertFile
	KeyFile string
	// ClientCertFile is a _client_ cert for initiating connections when ClientCertAuth is defined. If ClientCertAuth
	// is true but this value is empty, the CertFile will be used instead.
	ClientCertFile string
	// ClientKeyFile is the key for the ClientCertFile
	ClientKeyFile string

	TrustedCAFile       string
	ClientCertAuth      bool
	CRLFile             string
	InsecureSkipVerify  bool
	SkipClientSANVerify bool

	// ServerName ensures the cert matches the given host in case of discovery / virtual hosting
	ServerName string

	// HandshakeFailure is optionally called when a connection fails to handshake. The
	// connection will be closed immediately afterwards.
	HandshakeFailure func(*tls.Conn, error)

	// CipherSuites is a list of supported cipher suites.
	// If empty, Go auto-populates it by default.
	// Note that cipher suites are prioritized in the given order.
	CipherSuites []uint16

	// MinVersion is the minimum TLS version that is acceptable.
	// If not set, the minimum version is TLS 1.2.
	MinVersion uint16

	// MaxVersion is the maximum TLS version that is acceptable.
	// If not set, the default used by Go is selected (see tls.Config.MaxVersion).
	MaxVersion uint16

	selfCert bool

	// parseFunc exists to simplify testing. Typically, parseFunc
	// should be left nil. In that case, tls.X509KeyPair will be used.
	parseFunc func([]byte, []byte) (tls.Certificate, error)

	// AllowedCN is a CN which must be provided by a client.
	AllowedCN string

	// AllowedHostname is an IP address or hostname that must match the TLS
	// certificate provided by a client.
	AllowedHostname string

	// Logger logs TLS errors.
	// If nil, all logs are discarded.
	Logger *zap.Logger

	// EmptyCN indicates that the cert must have empty CN.
	// If true, ClientConfig() will return an error for a cert with non empty CN.
	EmptyCN bool
}

func (info TLSInfo) String() string {
	return fmt.Sprintf("cert = %s, key = %s, client-cert=%s, client-key=%s, trusted-ca = %s, client-cert-auth = %v, crl-file = %s", info.CertFile, info.KeyFile, info.ClientCertFile, info.ClientKeyFile, info.TrustedCAFile, info.ClientCertAuth, info.CRLFile)
}

func (info TLSInfo) Empty() bool {
	return info.CertFile == "" && info.KeyFile == ""
}

func SelfCert(lg *zap.Logger, dirpath string, hosts []string, selfSignedCertValidity uint, additionalUsages ...x509.ExtKeyUsage) (info TLSInfo, err error) {
	info.Logger = lg
	if selfSignedCertValidity == 0 {
		err = fmt.Errorf("selfSignedCertValidity is invalid,it should be greater than 0")
		info.Logger.Warn(
			"cannot generate cert",
			zap.Error(err),
		)
		return
	}
	err = fileutil.TouchDirAll(lg, dirpath)
	if err != nil {
		if info.Logger != nil {
			info.Logger.Warn(
				"cannot create cert directory",
				zap.Error(err),
			)
		}
		return
	}

	certPath, err := filepath.Abs(filepath.Join(dirpath, "cert.pem"))
	if err != nil {
		return
	}
	keyPath, err := filepath.Abs(filepath.Join(dirpath, "key.pem"))
	if err != nil {
		return
	}
	_, errcert := os.Stat(certPath)
	_, errkey := os.Stat(keyPath)
	if errcert == nil && errkey == nil {
		info.CertFile = certPath
		info.KeyFile = keyPath
		info.ClientCertFile = certPath
		info.ClientKeyFile = keyPath
		info.selfCert = true
		return
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		if info.Logger != nil {
			info.Logger.Warn(
				"cannot generate random number",
				zap.Error(err),
			)
		}
		return
	}

	tmpl := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{Organization: []string{"etcd"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Duration(selfSignedCertValidity) * 365 * (24 * time.Hour)),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           append([]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, additionalUsages...),
		BasicConstraintsValid: true,
	}

	if info.Logger != nil {
		info.Logger.Warn(
			"automatically generate certificates",
			zap.Time("certificate-validity-bound-not-after", tmpl.NotAfter),
		)
	}

	for _, host := range hosts {
		h, _, _ := net.SplitHostPort(host)
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		if info.Logger != nil {
			info.Logger.Warn(
				"cannot generate ECDSA key",
				zap.Error(err),
			)
		}
		return
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		if info.Logger != nil {
			info.Logger.Warn(
				"cannot generate x509 certificate",
				zap.Error(err),
			)
		}
		return
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		info.Logger.Warn(
			"cannot cert file",
			zap.String("path", certPath),
			zap.Error(err),
		)
		return
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()
	if info.Logger != nil {
		info.Logger.Info("created cert file", zap.String("path", certPath))
	}

	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		if info.Logger != nil {
			info.Logger.Warn(
				"cannot key file",
				zap.String("path", keyPath),
				zap.Error(err),
			)
		}
		return
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
	keyOut.Close()
	if info.Logger != nil {
		info.Logger.Info("created key file", zap.String("path", keyPath))
	}
	return SelfCert(lg, dirpath, hosts, selfSignedCertValidity)
}

// baseConfig is called on initial TLS handshake start.
//
// Previously,
// 1. Server has non-empty (*tls.Config).Certificates on client hello
// 2. Server calls (*tls.Config).GetCertificate iff:
//   - Server's (*tls.Config).Certificates is not empty, or
//   - Client supplies SNI; non-empty (*tls.ClientHelloInfo).ServerName
//
// When (*tls.Config).Certificates is always populated on initial handshake,
// client is expected to provide a valid matching SNI to pass the TLS
// verification, thus trigger server (*tls.Config).GetCertificate to reload
// TLS assets. However, a cert whose SAN field does not include domain names
// but only IP addresses, has empty (*tls.ClientHelloInfo).ServerName, thus
// it was never able to trigger TLS reload on initial handshake; first
// ceritifcate object was being used, never being updated.
//
// Now, (*tls.Config).Certificates is created empty on initial TLS client
// handshake, in order to trigger (*tls.Config).GetCertificate and populate
// rest of the certificates on every new TLS connection, even when client
// SNI is empty (e.g. cert only includes IPs).
func (info TLSInfo) baseConfig() (*tls.Config, error) {
	if info.KeyFile == "" || info.CertFile == "" {
		return nil, fmt.Errorf("KeyFile and CertFile must both be present[key: %v, cert: %v]", info.KeyFile, info.CertFile)
	}
	if info.Logger == nil {
		info.Logger = zap.NewNop()
	}

	_, err := tlsutil.NewCert(info.CertFile, info.KeyFile, info.parseFunc)
	if err != nil {
		return nil, err
	}

	// Perform prevalidation of client cert and key if either are provided. This makes sure we crash before accepting any connections.
	if (info.ClientKeyFile == "") != (info.ClientCertFile == "") {
		return nil, fmt.Errorf("ClientKeyFile and ClientCertFile must both be present or both absent: key: %v, cert: %v]", info.ClientKeyFile, info.ClientCertFile)
	}
	if info.ClientCertFile != "" {
		_, err := tlsutil.NewCert(info.ClientCertFile, info.ClientKeyFile, info.parseFunc)
		if err != nil {
			return nil, err
		}
	}

	var minVersion uint16
	if info.MinVersion != 0 {
		minVersion = info.MinVersion
	} else {
		// Default minimum version is TLS 1.2, previous versions are insecure and deprecated.
		minVersion = tls.VersionTLS12
	}

	cfg := &tls.Config{
		MinVersion: minVersion,
		MaxVersion: info.MaxVersion,
		ServerName: info.ServerName,
	}

	if len(info.CipherSuites) > 0 {
		cfg.CipherSuites = info.CipherSuites
	}

	// Client certificates may be verified by either an exact match on the CN,
	// or a more general check of the CN and SANs.
	var verifyCertificate func(*x509.Certificate) bool
	if info.AllowedCN != "" {
		if info.AllowedHostname != "" {
			return nil, fmt.Errorf("AllowedCN and AllowedHostname are mutually exclusive (cn=%q, hostname=%q)", info.AllowedCN, info.AllowedHostname)
		}
		verifyCertificate = func(cert *x509.Certificate) bool {
			return info.AllowedCN == cert.Subject.CommonName
		}
	}
	if info.AllowedHostname != "" {
		verifyCertificate = func(cert *x509.Certificate) bool {
			return cert.VerifyHostname(info.AllowedHostname) == nil
		}
	}
	if verifyCertificate != nil {
		cfg.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			for _, chains := range verifiedChains {
				if len(chains) != 0 {
					if verifyCertificate(chains[0]) {
						return nil
					}
				}
			}
			return errors.New("client certificate authentication failed")
		}
	}

	// this only reloads certs when there's a client request
	// TODO: support server-side refresh (e.g. inotify, SIGHUP), caching
	cfg.GetCertificate = func(clientHello *tls.ClientHelloInfo) (cert *tls.Certificate, err error) {
		cert, err = tlsutil.NewCert(info.CertFile, info.KeyFile, info.parseFunc)
		if os.IsNotExist(err) {
			if info.Logger != nil {
				info.Logger.Warn(
					"failed to find peer cert files",
					zap.String("cert-file", info.CertFile),
					zap.String("key-file", info.KeyFile),
					zap.Error(err),
				)
			}
		} else if err != nil {
			if info.Logger != nil {
				info.Logger.Warn(
					"failed to create peer certificate",
					zap.String("cert-file", info.CertFile),
					zap.String("key-file", info.KeyFile),
					zap.Error(err),
				)
			}
		}
		return cert, err
	}
	cfg.GetClientCertificate = func(unused *tls.CertificateRequestInfo) (cert *tls.Certificate, err error) {
		certfile, keyfile := info.CertFile, info.KeyFile
		if info.ClientCertFile != "" {
			certfile, keyfile = info.ClientCertFile, info.ClientKeyFile
		}
		cert, err = tlsutil.NewCert(certfile, keyfile, info.parseFunc)
		if os.IsNotExist(err) {
			if info.Logger != nil {
				info.Logger.Warn(
					"failed to find client cert files",
					zap.String("cert-file", certfile),
					zap.String("key-file", keyfile),
					zap.Error(err),
				)
			}
		} else if err != nil {
			if info.Logger != nil {
				info.Logger.Warn(
					"failed to create client certificate",
					zap.String("cert-file", certfile),
					zap.String("key-file", keyfile),
					zap.Error(err),
				)
			}
		}
		return cert, err
	}
	return cfg, nil
}

// cafiles returns a list of CA file paths.
func (info TLSInfo) cafiles() []string {
	cs := make([]string, 0)
	if info.TrustedCAFile != "" {
		cs = append(cs, info.TrustedCAFile)
	}
	return cs
}

// ServerConfig generates a tls.Config object for use by an HTTP server.
func (info TLSInfo) ServerConfig() (*tls.Config, error) {
	cfg, err := info.baseConfig()
	if err != nil {
		return nil, err
	}

	if info.Logger == nil {
		info.Logger = zap.NewNop()
	}

	cfg.ClientAuth = tls.NoClientCert
	if info.TrustedCAFile != "" || info.ClientCertAuth {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	cs := info.cafiles()
	if len(cs) > 0 {
		info.Logger.Info("Loading cert pool", zap.Strings("cs", cs),
			zap.Any("tlsinfo", info))
		cp, err := tlsutil.NewCertPool(cs)
		if err != nil {
			return nil, err
		}
		cfg.ClientCAs = cp
	}

	// "h2" NextProtos is necessary for enabling HTTP2 for go's HTTP server
	cfg.NextProtos = []string{"h2"}

	return cfg, nil
}

// ClientConfig generates a tls.Config object for use by an HTTP client.
func (info TLSInfo) ClientConfig() (*tls.Config, error) {
	var cfg *tls.Config
	var err error

	if !info.Empty() {
		cfg, err = info.baseConfig()
		if err != nil {
			return nil, err
		}
	} else {
		cfg = &tls.Config{ServerName: info.ServerName}
	}
	cfg.InsecureSkipVerify = info.InsecureSkipVerify

	cs := info.cafiles()
	if len(cs) > 0 {
		cfg.RootCAs, err = tlsutil.NewCertPool(cs)
		if err != nil {
			return nil, err
		}
	}

	if info.selfCert {
		cfg.InsecureSkipVerify = true
	}

	if info.EmptyCN {
		hasNonEmptyCN := false
		cn := ""
		_, err := tlsutil.NewCert(info.CertFile, info.KeyFile, func(certPEMBlock []byte, keyPEMBlock []byte) (tls.Certificate, error) {
			var block *pem.Block
			block, _ = pem.Decode(certPEMBlock)
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return tls.Certificate{}, err
			}
			if len(cert.Subject.CommonName) != 0 {
				hasNonEmptyCN = true
				cn = cert.Subject.CommonName
			}
			return tls.X509KeyPair(certPEMBlock, keyPEMBlock)
		})
		if err != nil {
			return nil, err
		}
		if hasNonEmptyCN {
			return nil, fmt.Errorf("cert has non empty Common Name (%s): %s", cn, info.CertFile)
		}
	}

	return cfg, nil
}

// IsClosedConnError returns true if the error is from closing listener, cmux.
// copied from golang.org/x/net/http2/http2.go
func IsClosedConnError(err error) bool {
	// 'use of closed network connection' (Go <=1.8)
	// 'use of closed file or network connection' (Go >1.8, internal/poll.ErrClosing)
	// 'mux: listener closed' (cmux.ErrListenerClosed)
	return err != nil && strings.Contains(err.Error(), "closed")
}
