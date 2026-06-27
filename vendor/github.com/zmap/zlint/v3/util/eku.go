package util

import (
	"fmt"
	"sort"

	"github.com/zmap/zcrypto/x509"
)

// HasEKU tests whether an Extended Key Usage (EKU) is present in a certificate.
func HasEKU(cert *x509.Certificate, eku x509.ExtKeyUsage) bool {
	for _, currentEku := range cert.ExtKeyUsage {
		if currentEku == eku {
			return true
		}
	}

	return false
}

// GetEKUString returns a human friendly Extended Key Usage (EKU) string.
func GetEKUString(eku x509.ExtKeyUsage) string {
	switch eku {
	case x509.ExtKeyUsageAny:
		return "any"
	case x509.ExtKeyUsageServerAuth:
		return "serverAuth"
	case x509.ExtKeyUsageClientAuth:
		return "clientAuth"
	case x509.ExtKeyUsageCodeSigning:
		return "codeSigning"
	case x509.ExtKeyUsageEmailProtection:
		return "emailProtection"
	case x509.ExtKeyUsageIpsecUser:
		return "ipSecUser"
	case x509.ExtKeyUsageIpsecTunnel:
		return "ipSecTunnel"
	case x509.ExtKeyUsageOcspSigning:
		return "ocspSigning"
	case x509.ExtKeyUsageMicrosoftServerGatedCrypto:
		return "microsoftServerGatedCrypto"
	case x509.ExtKeyUsageNetscapeServerGatedCrypto:
		return "netscapeServerGatedCrypto"
	}
	return fmt.Sprintf("unknown EKU %d", eku)
}

// GetEKUStrings returns a list of human friendly Extended Key Usage (EKU) strings.
func GetEKUStrings(eku []x509.ExtKeyUsage) []string {
	var ekuStrings []string
	for _, currentEku := range eku {
		ekuStrings = append(ekuStrings, GetEKUString(currentEku))
	}
	sort.Strings(ekuStrings)
	return ekuStrings
}
