package rfc

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
	"fmt"
	"sort"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type KUAndEKUInconsistent struct{}

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_key_usage_and_extended_key_usage_inconsistent",
			Description:   "The certificate MUST only be used for a purpose consistent with both key usage extension and extended key usage extension.",
			Citation:      "RFC 5280, Section 4.2.1.12.",
			Source:        lint.RFC5280,
			EffectiveDate: util.RFC5280Date,
		},
		Lint: NewKUAndEKUInconsistent,
	})
}

func NewKUAndEKUInconsistent() lint.LintInterface {
	return &KUAndEKUInconsistent{}
}

func (l *KUAndEKUInconsistent) Initialize() error {
	return nil
}

// CheckApplies returns true when the certificate contains both a key usage
// extension and an extended key usage extension.
func (l *KUAndEKUInconsistent) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.IsExtInCert(c, util.EkuSynOid) && util.IsExtInCert(c, util.KeyUsageOID)
}

// Execute returns an Error level lint.LintResult if the purposes of the certificate
// being linted is not consistent with both extensions.
func (l *KUAndEKUInconsistent) Execute(c *x509.Certificate) *lint.LintResult {
	if len(c.ExtKeyUsage) > 1 {
		return l.multiPurpose(c)
	}
	return l.strictPurpose(c)
}

// RFC 5280 4.2.1.12 on multiple purposes:
//
//	If multiple purposes are indicated the application need not recognize all purposes
//	indicated, as long as the intended purpose is present.
func (l *KUAndEKUInconsistent) multiPurpose(c *x509.Certificate) *lint.LintResult {
	// Create a map with each KeyUsage combination that is authorized for the
	// included extKeyUsage(es).
	var mp = map[x509.KeyUsage]bool{}
	for _, extKeyUsage := range c.ExtKeyUsage {
		var i int
		if _, ok := eku[extKeyUsage]; !ok {
			return &lint.LintResult{Status: lint.Pass}
		}
		for ku := range eku[extKeyUsage] {
			// There is nothing to merge for the first EKU.
			if i > 0 {
				// We could see this EKU combined with any other EKU so
				// create that possibility.
				for mpku := range mp {
					mp[mpku|ku] = true
				}
			}

			mp[ku] = true
			i++
		}
	}
	if !mp[c.KeyUsage] {
		// Sort the included KeyUsage strings for consistent error messages
		// The order does not matter for this lint, but the consistency makes
		// it easier to identify common errors.
		keyUsage := util.GetKeyUsageStrings(c.KeyUsage)
		sort.Strings(keyUsage)

		return &lint.LintResult{
			Status:  lint.Error,
			Details: fmt.Sprintf("KeyUsage %v (%08b) inconsistent with multiple purpose ExtKeyUsage %v", keyUsage, c.KeyUsage, util.GetEKUStrings(c.ExtKeyUsage)),
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}

// strictPurpose checks if the Key Usages (KU) included are permitted for each
// indicated Extended Key Usage (EKU)
func (l *KUAndEKUInconsistent) strictPurpose(c *x509.Certificate) *lint.LintResult {
	for _, extKeyUsage := range c.ExtKeyUsage {
		if _, ok := eku[extKeyUsage]; !ok {
			continue
		}
		if !eku[extKeyUsage][c.KeyUsage] {
			return &lint.LintResult{
				Status:  lint.Error,
				Details: fmt.Sprintf("KeyUsage %v (%08b) inconsistent with ExtKeyUsage %s", util.GetKeyUsageStrings(c.KeyUsage), c.KeyUsage, util.GetEKUString(extKeyUsage)),
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}

var eku = map[x509.ExtKeyUsage]map[x509.KeyUsage]bool{

	// KU combinations with Server Authentication EKU:
	//  RFC 5280 4.2.1.12 on KU consistency with Server Authentication EKU:
	//    -- TLS WWW server authentication
	//    -- Key usage bits that may be consistent: digitalSignature,
	//    -- keyEncipherment or keyAgreement

	// (digitalSignature OR (keyEncipherment XOR keyAgreement))
	x509.ExtKeyUsageServerAuth: {
		x509.KeyUsageDigitalSignature:                                true,
		x509.KeyUsageKeyEncipherment:                                 true,
		x509.KeyUsageKeyAgreement:                                    true,
		x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment: true,
		x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement:    true,
	},

	// KU combinations with Client Authentication EKU:
	//  RFC 5280 4.2.1.12 on KU consistency with Client Authentication EKU:
	//    -- TLS WWW client authentication
	//    -- Key usage bits that may be consistent: digitalSignature
	//    -- and/or keyAgreement

	// (digitalSignature OR keyAgreement)
	x509.ExtKeyUsageClientAuth: {
		x509.KeyUsageDigitalSignature:                             true,
		x509.KeyUsageKeyAgreement:                                 true,
		x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement: true,
	},

	// KU combinations with Code Signing EKU:
	//  RFC 5280 4.2.1.12 on KU consistency with Code Signing EKU:
	//   -- Signing of downloadable executable code
	//   -- Key usage bits that may be consistent: digitalSignature

	// (digitalSignature)
	x509.ExtKeyUsageCodeSigning: {
		x509.KeyUsageDigitalSignature: true,
	},

	// KU combinations with Email Protection EKU:
	//  RFC 5280 4.2.1.12 on KU consistency with Email Protection EKU:
	//    -- Email protection
	//    -- Key usage bits that may be consistent: digitalSignature,
	//    -- nonRepudiation, and/or (keyEncipherment or keyAgreement)
	//  Note: Recent editions of X.509 have renamed nonRepudiation bit to contentCommitment

	// (digitalSignature OR nonRepudiation OR (keyEncipherment XOR keyAgreement))
	x509.ExtKeyUsageEmailProtection: {
		x509.KeyUsageDigitalSignature:  true,
		x509.KeyUsageContentCommitment: true,
		x509.KeyUsageKeyEncipherment:   true,
		x509.KeyUsageKeyAgreement:      true,

		x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment: true,
		x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment:   true,
		x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement:      true,

		x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment: true,
		x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment | x509.KeyUsageKeyAgreement:    true,

		x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment: true,
		x509.KeyUsageContentCommitment | x509.KeyUsageKeyAgreement:    true,
	},

	// KU combinations with Time Stamping EKU:
	//  RFC 5280 4.2.1.12 on KU consistency with Time Stamping EKU:
	//    -- Binding the hash of an object to a time
	//    -- Key usage bits that may be consistent: digitalSignature
	//    -- and/or nonRepudiation
	//  Note: Recent editions of X.509 have renamed nonRepudiation bit to contentCommitment

	// (digitalSignature OR nonRepudiation)
	x509.ExtKeyUsageTimeStamping: {
		x509.KeyUsageDigitalSignature:                                  true,
		x509.KeyUsageContentCommitment:                                 true,
		x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment: true,
	},

	// KU combinations with Ocsp Signing EKU:
	//  RFC 5280 4.2.1.12 on KU consistency with Ocsp Signing EKU:
	//    -- Signing OCSP responses
	//    -- Key usage bits that may be consistent: digitalSignature
	//    -- and/or nonRepudiation
	//  Note: Recent editions of X.509 have renamed nonRepudiation bit to contentCommitment

	// (digitalSignature OR nonRepudiation)
	x509.ExtKeyUsageOcspSigning: {
		x509.KeyUsageDigitalSignature:                                  true,
		x509.KeyUsageContentCommitment:                                 true,
		x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment: true,
	},
}
