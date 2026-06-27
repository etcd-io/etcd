package util

/*
 * ZLint Copyright 2021 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

import (
	"github.com/zmap/zcrypto/x509"
)

func IsMailboxValidatedCertificate(c *x509.Certificate) bool {
	for _, oid := range c.PolicyIdentifiers {
		if oid.Equal(SMIMEBRMailboxValidatedLegacyOID) || oid.Equal(SMIMEBRMailboxValidatedMultipurposeOID) || oid.Equal(SMIMEBRMailboxValidatedStrictOID) {
			return true
		}
	}

	return false
}

func IsSMIMEBRCertificate(c *x509.Certificate) bool {
	return IsLegacySMIMECertificate(c) || IsMultipurposeSMIMECertificate(c) || IsStrictSMIMECertificate(c)
}

func IsLegacySMIMECertificate(c *x509.Certificate) bool {
	for _, oid := range c.PolicyIdentifiers {
		if oid.Equal(SMIMEBRMailboxValidatedLegacyOID) || oid.Equal(SMIMEBROrganizationValidatedLegacyOID) || oid.Equal(SMIMEBRSponsorValidatedLegacyOID) || oid.Equal(SMIMEBRIndividualValidatedLegacyOID) {
			return true
		}
	}

	return false
}

func IsOrganizationValidatedCertificate(c *x509.Certificate) bool {
	for _, oid := range c.PolicyIdentifiers {
		if oid.Equal(SMIMEBROrganizationValidatedLegacyOID) || oid.Equal(SMIMEBROrganizationValidatedMultipurposeOID) || oid.Equal(SMIMEBROrganizationValidatedStrictOID) {
			return true
		}
	}

	return false
}

func IsSponsorValidatedCertificate(c *x509.Certificate) bool {
	for _, oid := range c.PolicyIdentifiers {
		if oid.Equal(SMIMEBRSponsorValidatedLegacyOID) || oid.Equal(SMIMEBRSponsorValidatedMultipurposeOID) || oid.Equal(SMIMEBRSponsorValidatedStrictOID) {
			return true
		}
	}

	return false
}

func IsMultipurposeSMIMECertificate(c *x509.Certificate) bool {
	for _, oid := range c.PolicyIdentifiers {
		if oid.Equal(SMIMEBRMailboxValidatedMultipurposeOID) || oid.Equal(SMIMEBROrganizationValidatedMultipurposeOID) || oid.Equal(SMIMEBRSponsorValidatedMultipurposeOID) || oid.Equal(SMIMEBRIndividualValidatedMultipurposeOID) {
			return true
		}
	}

	return false
}

func IsStrictSMIMECertificate(c *x509.Certificate) bool {
	for _, oid := range c.PolicyIdentifiers {
		if oid.Equal(SMIMEBRMailboxValidatedStrictOID) || oid.Equal(SMIMEBROrganizationValidatedStrictOID) || oid.Equal(SMIMEBRSponsorValidatedStrictOID) || oid.Equal(SMIMEBRIndividualValidatedStrictOID) {
			return true
		}
	}

	return false
}
