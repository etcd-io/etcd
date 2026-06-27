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
			Name:          "e_rsa_other_key_usages",
			Description:   "Other bit positions SHALL NOT be set.",
			Citation:      "7.1.2.3.e",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewRSAOtherKeyUsages,
	})
}

type rsaOtherKeyUsages struct{}

func NewRSAOtherKeyUsages() lint.LintInterface {
	return &rsaOtherKeyUsages{}
}

func (l *rsaOtherKeyUsages) CheckApplies(c *x509.Certificate) bool {
	if !(util.IsSubscriberCert(c) && util.IsSMIMEBRCertificate(c) && util.IsExtInCert(c, util.KeyUsageOID)) {
		return false
	}

	_, ok := c.PublicKey.(*rsa.PublicKey)
	return ok && c.PublicKeyAlgorithm == x509.RSA
}

func (l *rsaOtherKeyUsages) Execute(c *x509.Certificate) *lint.LintResult {
	if !(util.HasKeyUsage(c, x509.KeyUsageDigitalSignature) || util.HasKeyUsage(c, x509.KeyUsageKeyEncipherment)) {
		if c.KeyUsage != 0 {
			return &lint.LintResult{Status: lint.Error}
		}

		return &lint.LintResult{Status: lint.NA}
	}

	return &lint.LintResult{Status: lint.Pass}
}
