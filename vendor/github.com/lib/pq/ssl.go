package pq

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/lib/pq/internal/pqutil"
)

// Registry for custom tls.Configs
var (
	tlsConfs   = make(map[string]*tls.Config)
	tlsConfsMu sync.RWMutex
)

// RegisterTLSConfig registers a custom [tls.Config]. They are used by using
// sslmode=pqgo-«key» in the connection string.
//
// Set the config to nil to remove a configuration.
func RegisterTLSConfig(key string, config *tls.Config) error {
	key = strings.TrimPrefix(key, "pqgo-")
	if config == nil {
		tlsConfsMu.Lock()
		delete(tlsConfs, key)
		tlsConfsMu.Unlock()
		return nil
	}

	tlsConfsMu.Lock()
	tlsConfs[key] = config
	tlsConfsMu.Unlock()
	return nil
}

func hasTLSConfig(key string) bool {
	tlsConfsMu.RLock()
	defer tlsConfsMu.RUnlock()
	_, ok := tlsConfs[key]
	return ok
}

func getTLSConfigClone(key string) *tls.Config {
	tlsConfsMu.RLock()
	defer tlsConfsMu.RUnlock()
	if v, ok := tlsConfs[key]; ok {
		return v.Clone()
	}
	return nil
}

// ssl generates a function to upgrade a net.Conn based on the "sslmode" and
// related settings. The function is nil when no upgrade should take place.
//
// Don't refer to Config.SSLMode here, as the mode in arguments may be different
// in case of sslmode=allow or prefer.
func ssl(cfg Config, mode SSLMode) (func(net.Conn) (net.Conn, error), error) {
	var (
		home = pqutil.Home(true)
		// Don't set defaults here, because tlsConf may be overwritten if a
		// custom one was registered. Set it after the sslmode switch.
		tlsConf = &tls.Config{}
		// Only verify the CA signing but not the hostname.
		verifyCaOnly = false
	)
	if mode.useSSL() && !cfg.SSLInline && cfg.SSLRootCert == "" && home != "" {
		f := filepath.Join(home, "root.crt")
		if _, err := os.Stat(f); err == nil {
			cfg.SSLRootCert = f
		}
	}
	switch {
	case mode == SSLModeDisable || mode == SSLModeAllow:
		return nil, nil

	case mode == "" || mode == SSLModeRequire || mode == SSLModePrefer:
		// Skip TLS's own verification since it requires full verification.
		tlsConf.InsecureSkipVerify = true

		// From http://www.postgresql.org/docs/current/static/libpq-ssl.html:
		//
		// For backwards compatibility with earlier versions of PostgreSQL, if a
		// root CA file exists, the behavior of sslmode=require will be the same
		// as that of verify-ca, meaning the server certificate is validated
		// against the CA. Relying on this behavior is discouraged, and
		// applications that need certificate validation should always use
		// verify-ca or verify-full.
		if cfg.SSLRootCert != "" {
			if cfg.SSLInline {
				verifyCaOnly = true
			} else if _, err := os.Stat(cfg.SSLRootCert); err == nil {
				verifyCaOnly = true
			} else if cfg.SSLRootCert != "system" {
				cfg.SSLRootCert = ""
			}
		}
	case mode == SSLModeVerifyCA:
		// Skip TLS's own verification since it requires full verification.
		tlsConf.InsecureSkipVerify = true
		verifyCaOnly = true
	case mode == SSLModeVerifyFull:
		tlsConf.ServerName = cfg.Host
	case strings.HasPrefix(string(mode), "pqgo-"):
		tlsConf = getTLSConfigClone(string(mode[5:]))
		if tlsConf == nil {
			return nil, fmt.Errorf(`pq: unknown custom sslmode %q`, mode)
		}
	default:
		panic("unreachable")
	}

	tlsConf.MinVersion = cfg.SSLMinProtocolVersion.tlsconf()
	tlsConf.MaxVersion = cfg.SSLMaxProtocolVersion.tlsconf()

	// RFC 6066 asks to not set SNI if the host is a literal IP address (IPv4 or
	// IPv6). This check is coded already crypto.tls.hostnameInSNI, so just
	// always set ServerName here and let crypto/tls do the filtering.
	if cfg.SSLSNI {
		tlsConf.ServerName = cfg.Host
	}

	err := sslClientCertificates(tlsConf, cfg, home)
	if err != nil {
		return nil, err
	}
	rootPem, err := sslCertificateAuthority(tlsConf, cfg)
	if err != nil {
		return nil, err
	}
	sslAppendIntermediates(tlsConf, cfg, rootPem)

	// Accept renegotiation requests initiated by the backend.
	//
	// Renegotiation was deprecated then removed from PostgreSQL 9.5, but the
	// default configuration of older versions has it enabled. Redshift also
	// initiates renegotiations and cannot be reconfigured.
	//
	// TODO: I think this can be removed?
	tlsConf.Renegotiation = tls.RenegotiateFreelyAsClient

	return func(conn net.Conn) (net.Conn, error) {
		client := tls.Client(conn, tlsConf)
		if verifyCaOnly {
			err := client.Handshake()
			if err != nil {
				return client, err
			}
			var (
				certs = client.ConnectionState().PeerCertificates
				opts  = x509.VerifyOptions{Intermediates: x509.NewCertPool(), Roots: tlsConf.RootCAs}
			)
			for _, cert := range certs[1:] {
				opts.Intermediates.AddCert(cert)
			}
			_, err = certs[0].Verify(opts)
			return client, err
		}
		return client, nil
	}, nil
}

// sslClientCertificates adds the certificate specified in the "sslcert" and
//
// "sslkey" settings, or if they aren't set, from the .postgresql directory
// in the user's home directory. The configured files must exist and have
// the correct permissions.
func sslClientCertificates(tlsConf *tls.Config, cfg Config, home string) error {
	if cfg.SSLInline {
		cert, err := tls.X509KeyPair([]byte(cfg.SSLCert), []byte(cfg.SSLKey))
		if err != nil {
			return err
		}
		// Use GetClientCertificate instead of the Certificates field. When
		// Certificates is set, Go's TLS client only sends the cert if the
		// server's CertificateRequest includes a CA that issued it. When the
		// client cert was signed by an intermediate CA but the server only
		// advertises the root CA, Go skips sending the cert entirely.
		// GetClientCertificate bypasses this filtering.
		tlsConf.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &cert, nil
		}
		return nil
	}

	// Only load client certificate and key if the setting is not blank, like libpq.
	if cfg.SSLCert == "" && home != "" {
		cfg.SSLCert = filepath.Join(home, "postgresql.crt")
	}
	if cfg.SSLCert == "" {
		return nil
	}
	_, err := os.Stat(cfg.SSLCert)
	if err != nil {
		if pqutil.ErrNotExists(err) {
			return nil
		}
		return err
	}

	// In libpq, the ssl key is only loaded if the setting is not blank.
	if cfg.SSLKey == "" && home != "" {
		cfg.SSLKey = filepath.Join(home, "postgresql.key")
	}
	if cfg.SSLKey != "" {
		err := pqutil.SSLKeyPermissions(cfg.SSLKey)
		if err != nil {
			return err
		}
	}

	cert, err := tls.LoadX509KeyPair(cfg.SSLCert, cfg.SSLKey)
	if err != nil {
		return err
	}

	// Using GetClientCertificate instead of Certificates per comment above.
	tlsConf.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return &cert, nil
	}
	return nil
}

var testSystemRoots *x509.CertPool

// sslCertificateAuthority adds the RootCA specified in the "sslrootcert" setting.
func sslCertificateAuthority(tlsConf *tls.Config, cfg Config) ([]byte, error) {
	// Only load root certificate if not blank, like libpq.
	if cfg.SSLRootCert == "" {
		return nil, nil
	}

	if cfg.SSLRootCert == "system" {
		// No work to do as system CAs are used by default if RootCAs is nil.
		tlsConf.RootCAs = testSystemRoots
		return nil, nil
	}

	tlsConf.RootCAs = x509.NewCertPool()

	var cert []byte
	if cfg.SSLInline {
		cert = []byte(cfg.SSLRootCert)
	} else {
		var err error
		cert, err = os.ReadFile(cfg.SSLRootCert)
		if err != nil {
			return nil, err
		}
	}

	if !tlsConf.RootCAs.AppendCertsFromPEM(cert) {
		return nil, errors.New("pq: couldn't parse pem from sslrootcert")
	}
	return cert, nil
}

// sslAppendIntermediates appends intermediate CA certificates from sslrootcert
// to the client certificate chain. This is needed so the server can verify the
// client cert when it was signed by an intermediate CA — without this, the TLS
// handshake only sends the leaf client cert.
func sslAppendIntermediates(tlsConf *tls.Config, cfg Config, rootPem []byte) {
	if cfg.SSLRootCert == "" || tlsConf.GetClientCertificate == nil || len(rootPem) == 0 {
		return
	}

	var (
		pemData       = slices.Clone(rootPem)
		intermediates [][]byte
	)
	for {
		var block *pem.Block
		block, pemData = pem.Decode(pemData)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		// Skip self-signed root CAs; only append intermediates.
		if cert.IsCA && !bytes.Equal(cert.RawIssuer, cert.RawSubject) {
			intermediates = append(intermediates, block.Bytes)
		}
	}
	if len(intermediates) == 0 {
		return
	}

	// Wrap the existing GetClientCertificate to append intermediate certs to
	// the certificate chain returned during the TLS handshake.
	origGetCert := tlsConf.GetClientCertificate
	tlsConf.GetClientCertificate = func(info *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		cert, err := origGetCert(info)
		if err != nil {
			return cert, err
		}
		cert.Certificate = append(cert.Certificate, intermediates...)
		return cert, nil
	}
}
