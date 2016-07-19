// Package revoke provides functionality for checking the validity of
// a cert. Specifically, the temporal validity of the certificate is
// checked first, then any CRL and OCSP url in the cert is checked.
package revoke

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"os"
	"time"

	"golang.org/x/crypto/ocsp"

	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/log"
)

// Revoke type contains configuration for each new revoke instance
type Revoke struct {
	// HardFail determines whether the failure to check the revocation
	// status of a certificate (i.e. due to network failure) causes
	// verification to fail (a hard failure).
	HardFail bool
	// CRLSet associates a PKIX certificate list with the URL the CRL is
	// fetched from.
	CRLSet map[string]*pkix.CertificateList
}

// NewRevokeChecker creates Revoke config structure
func NewRevokeChecker() *Revoke {
	return &Revoke{false, map[string]*pkix.CertificateList{}}
}

// We can't handle LDAP certificates, so this checks to see if the
// URL string points to an LDAP resource so that we can ignore it.
func ldapURL(url string) bool {
	u, err := neturl.Parse(url)
	if err != nil {
		log.Warningf("error parsing url %s: %v", url, err)
		return false
	}
	if u.Scheme == "ldap" {
		return true
	}
	return false
}

// revCheck should check the certificate for any revocations. It
// returns a pair of booleans: the first indicates whether the certificate
// is revoked, the second indicates whether the revocations were
// successfully checked.. This leads to the following combinations:
//
//  false, false: an error was encountered while checking revocations.
//
//  false, true:  the certificate was checked successfully and
//                  it is not revoked.
//
//  true, true:   the certificate was checked successfully and
//                  it is revoked.
//
//  true, false:  failure to check revocation status causes
//                  verification to fail
func (r *Revoke) revCheck(cert *x509.Certificate, localCRLPath string) (revoked, ok bool) {
	if localCRLPath != "" {
		if revoked, ok := r.certIsRevokedCRL(cert, localCRLPath, true); !ok {
			log.Warning("error checking revocation via CRL")
			if r.HardFail {
				return true, false
			}
			return false, false
		} else if revoked {
			log.Info("certificate is revoked via CRL")
			return true, true
		}
	}

	for _, url := range cert.CRLDistributionPoints {
		if ldapURL(url) {
			log.Infof("skipping LDAP CRL: %s", url)
			continue
		}

		if revoked, ok := r.certIsRevokedCRL(cert, url, false); !ok {
			log.Warning("error checking revocation via CRL")
			if r.HardFail {
				return true, false
			}
			return false, false
		} else if revoked {
			log.Info("certificate is revoked via CRL")
			return true, true
		}

		if revoked, ok := certIsRevokedOCSP(cert, r.HardFail); !ok {
			log.Warning("error checking revocation via OCSP")
			if r.HardFail {
				return true, false
			}
			return false, false
		} else if revoked {
			log.Info("certificate is revoked via OCSP")
			return true, true
		}
	}

	return false, true
}

// fetchCRL fetches and parses a CRL.
func fetchCRL(url string) (*pkix.CertificateList, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	} else if resp.StatusCode >= 300 {
		return nil, errors.New("failed to retrieve CRL")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	return x509.ParseCRL(body)
}

func getIssuer(cert *x509.Certificate) *x509.Certificate {
	var issuer *x509.Certificate
	var err error
	for _, issuingCert := range cert.IssuingCertificateURL {
		issuer, err = fetchRemote(issuingCert)
		if err != nil {
			continue
		}
		break
	}

	return issuer
}

// checks whether CRL in memory is valid
func (r *Revoke) isInMemoryCRLValid(key string) bool {
	crl, ok := r.CRLSet[key]
	if ok && crl == nil {
		ok = false
		delete(r.CRLSet, key)
	}

	if ok {
		if !crl.HasExpired(time.Now()) {
			return true
		}
	}

	return false
}

// FetchLocalCRL reads CRL from the local filesystem
// force flag allows you to update the CRL
func (r *Revoke) FetchLocalCRL(path string, force bool) error {
	shouldFetchCRL := !r.isInMemoryCRLValid(path)

	u, err := neturl.Parse(path)
	if err != nil {
		log.Warningf("failed to parse CRL url: %v", err)
		return err
	}

	if u.Scheme == "" && (shouldFetchCRL || force) {
		if _, err := os.Stat(path); err == nil {
			tmp, err := ioutil.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read local CRL path: %v", err)
			}
			crl, err := x509.ParseCRL(tmp)
			if err != nil {
				return fmt.Errorf("failed to parse local CRL file: %v", err)
			}
			r.CRLSet[path] = crl
		} else {
			return fmt.Errorf("failed to read local CRL path: %v", err)
		}
	}

	return nil
}

// FetchRemoteCRL fetches remote CRL into internal map,
// force overwrites previously read CRL
func (r *Revoke) FetchRemoteCRL(url string, cert *x509.Certificate, force bool) error {
	shouldFetchCRL := !r.isInMemoryCRLValid(url)

	issuer := getIssuer(cert)

	if force || shouldFetchCRL {
		var err error
		crl, err := fetchCRL(url)
		if err != nil {
			return fmt.Errorf("failed to fetch CRL: %v", err)
		}

		// check CRL signature
		if issuer != nil {
			err = issuer.CheckCRLSignature(crl)
			if err != nil {
				return fmt.Errorf("failed to verify CRL: %v", err)
			}
		}

		r.CRLSet[url] = crl
	}

	return nil
}

// check a cert against a specific CRL. Returns the same bool pair
// as revCheck.
func (r *Revoke) certIsRevokedCRL(cert *x509.Certificate, url string, fetchLocal bool) (revoked, ok bool) {
	var err error
	if fetchLocal {
		err = r.FetchLocalCRL(url, false)
	} else {
		err = r.FetchRemoteCRL(url, cert, false)
	}

	if err != nil {
		log.Warningf("%v", err)
		return false, false
	}

	for _, revoked := range r.CRLSet[url].TBSCertList.RevokedCertificates {
		if cert.SerialNumber.Cmp(revoked.SerialNumber) == 0 {
			log.Info("Serial number match: intermediate is revoked.")
			return true, true
		}
	}

	return false, true
}

// verifyCertTime verifies whether certificate time frames are valid
func verifyCertTime(cert *x509.Certificate) bool {
	if !time.Now().Before(cert.NotAfter) {
		log.Infof("Certificate expired %s", cert.NotAfter)
		return false
	} else if !time.Now().After(cert.NotBefore) {
		log.Infof("Certificate isn't valid until %s", cert.NotBefore)
		return false
	}

	return true
}

// VerifyCertificate ensures that the certificate passed in hasn't
// expired and checks the CRL for the server.
func (r *Revoke) VerifyCertificate(cert *x509.Certificate) (revoked, ok bool) {
	if !verifyCertTime(cert) {
		return true, true
	}

	return r.revCheck(cert, "")
}

// VerifyCertificateByCRLPath ensures that the certificate passed in hasn't
// expired and checks the local CRL path.
func (r *Revoke) VerifyCertificateByCRLPath(cert *x509.Certificate, crlPath string) (revoked, ok bool) {
	if !verifyCertTime(cert) {
		return true, true
	}

	return r.revCheck(cert, crlPath)
}

func fetchRemote(url string) (*x509.Certificate, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	in, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	p, _ := pem.Decode(in)
	if p != nil {
		return helpers.ParseCertificatePEM(in)
	}

	return x509.ParseCertificate(in)
}

var ocspOpts = ocsp.RequestOptions{
	Hash: crypto.SHA1,
}

func certIsRevokedOCSP(leaf *x509.Certificate, strict bool) (revoked, ok bool) {
	var err error

	ocspURLs := leaf.OCSPServer
	if len(ocspURLs) == 0 {
		// OCSP not enabled for this certificate.
		return false, true
	}

	issuer := getIssuer(leaf)

	if issuer == nil {
		return false, false
	}

	ocspRequest, err := ocsp.CreateRequest(leaf, issuer, &ocspOpts)
	if err != nil {
		return
	}

	for _, server := range ocspURLs {
		resp, err := sendOCSPRequest(server, ocspRequest, issuer)
		if err != nil {
			if strict {
				return
			}
			continue
		}
		if err = resp.CheckSignatureFrom(issuer); err != nil {
			return false, false
		}

		// There wasn't an error fetching the OCSP status.
		ok = true

		if resp.Status != ocsp.Good {
			// The certificate was revoked.
			revoked = true
		}

		return
	}
	return
}

// sendOCSPRequest attempts to request an OCSP response from the
// server. The error only indicates a failure to *fetch* the
// certificate, and *does not* mean the certificate is valid.
func sendOCSPRequest(server string, req []byte, issuer *x509.Certificate) (*ocsp.Response, error) {
	var resp *http.Response
	var err error
	if len(req) > 256 {
		buf := bytes.NewBuffer(req)
		resp, err = http.Post(server, "application/ocsp-request", buf)
	} else {
		reqURL := server + "/" + base64.StdEncoding.EncodeToString(req)
		resp, err = http.Get(reqURL)
	}

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to retrieve OSCP")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	switch {
	case bytes.Equal(body, ocsp.UnauthorizedErrorResponse):
		return nil, errors.New("OSCP unauthorized")
	case bytes.Equal(body, ocsp.MalformedRequestErrorResponse):
		return nil, errors.New("OSCP malformed")
	case bytes.Equal(body, ocsp.InternalErrorErrorResponse):
		return nil, errors.New("OSCP internal error")
	case bytes.Equal(body, ocsp.TryLaterErrorResponse):
		return nil, errors.New("OSCP try later")
	case bytes.Equal(body, ocsp.SigRequredErrorResponse):
		return nil, errors.New("OSCP signature required")
	}

	return ocsp.ParseResponse(body, issuer)
}
