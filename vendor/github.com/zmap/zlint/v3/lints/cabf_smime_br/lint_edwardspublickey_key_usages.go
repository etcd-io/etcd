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
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_edwardspublickey_key_usages",
			Description:   "Bit positions SHALL be set for digitalSignature and MAY be set for nonRepudiation.",
			Citation:      "7.1.2.3.e",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewEdwardsPublicKeyKeyUsages,
	})
}

type edwardsPublicKeyKeyUsages struct{}

func NewEdwardsPublicKeyKeyUsages() lint.LintInterface {
	return &edwardsPublicKeyKeyUsages{}
}

func (l *edwardsPublicKeyKeyUsages) CheckApplies(c *x509.Certificate) bool {
	// TODO add support for curve448 certificate linting
	return util.IsSubscriberCert(c) && util.IsSMIMEBRCertificate(c) && util.IsExtInCert(c, util.KeyUsageOID) && c.PublicKeyAlgorithm == x509.Ed25519
}

func (l *edwardsPublicKeyKeyUsages) Execute(c *x509.Certificate) *lint.LintResult {
	if !util.HasKeyUsage(c, x509.KeyUsageDigitalSignature) {
		return &lint.LintResult{Status: lint.Error}
	}

	mask := 0x1FF ^ (x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment)
	if c.KeyUsage&mask != 0 {
		return &lint.LintResult{Status: lint.Error}
	}

	return &lint.LintResult{Status: lint.Pass}
}
