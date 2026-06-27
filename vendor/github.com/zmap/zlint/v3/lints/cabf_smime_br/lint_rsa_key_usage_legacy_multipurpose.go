/*
 * ZLint Copyright 2023 Regents of the University of Michigan
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

package cabf_smime_br

import (
	"crypto/rsa"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_rsa_key_usage_legacy_multipurpose",
			Description:   "For signing only, bit positions SHALL be set for digitalSignature and MAY be set for nonRepudiation. For key management only, bit positions SHALL be set for keyEncipherment and MAY be set for dataEncipherment. For dual use, bit positions SHALL be set for digitalSignature and keyEncipherment and MAY be set for nonRepudiation and dataEncipherment.",
			Citation:      "7.1.2.3.e",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewRSAKeyUsageLegacyMultipurpose,
	})
}

type rsaKeyUsageLegacyMultipurpose struct{}

func NewRSAKeyUsageLegacyMultipurpose() lint.LintInterface {
	return &rsaKeyUsageLegacyMultipurpose{}
}

func (l *rsaKeyUsageLegacyMultipurpose) CheckApplies(c *x509.Certificate) bool {
	if !(util.IsSubscriberCert(c) && (util.IsLegacySMIMECertificate(c) || util.IsMultipurposeSMIMECertificate(c)) && util.IsExtInCert(c, util.KeyUsageOID)) {
		return false
	}

	_, ok := c.PublicKey.(*rsa.PublicKey)
	return ok && c.PublicKeyAlgorithm == x509.RSA
}

func (l *rsaKeyUsageLegacyMultipurpose) Execute(c *x509.Certificate) *lint.LintResult {
	const (
		signing = iota + 1
		keyManagement
		dualUsage
	)

	certType := 0
	if util.HasKeyUsage(c, x509.KeyUsageDigitalSignature) {
		certType |= signing
	}
	if util.HasKeyUsage(c, x509.KeyUsageKeyEncipherment) {
		certType |= keyManagement
	}

	switch certType {
	case signing:
		mask := 0x1FF ^ (x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment)
		if c.KeyUsage&mask != 0 {
			return &lint.LintResult{Status: lint.Error}
		}

	case keyManagement:
		mask := 0x1FF ^ (x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment)
		if c.KeyUsage&mask != 0 {
			return &lint.LintResult{Status: lint.Error}
		}

	case dualUsage:
		mask := 0x1FF ^ (x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment)
		if c.KeyUsage&mask != 0 {
			return &lint.LintResult{Status: lint.Error}
		}

	default:
		return &lint.LintResult{Status: lint.NA}
	}

	return &lint.LintResult{Status: lint.Pass}
}
