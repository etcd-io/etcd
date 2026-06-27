package cabf_smime_br

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

import (
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

// strictEKUCheck - linter to enforce requirement that SMIME certificates SHALL contain emailProtecton EKU
type strictEKUCheck struct {
}

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_smime_strict_eku_check",
			Description:   "Strict: id-kp-emailProtection SHALL be present.  Other values SHALL NOT be present",
			Citation:      "SMIME BRs: 7.1.2.3.f",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewStrictEKUCheck,
	})
}

// NewShallHaveCrlDistributionPoints creates a new linter to enforce MAY/SHALL NOT field requirements for mailbox validated SMIME certs
func NewStrictEKUCheck() lint.CertificateLintInterface {
	return &strictEKUCheck{}
}

// CheckApplies returns true if the provided certificate contains one-or-more of the following SMIME BR policy identifiers:
//   - Mailbox Validated Strict
//   - Organization Validated Strict
//   - Sponsor Validated Strict
//   - Individual Validated Strict
func (l *strictEKUCheck) CheckApplies(c *x509.Certificate) bool {
	return util.IsStrictSMIMECertificate(c) && util.IsSubscriberCert(c)
}

// Execute applies the requirements on what fields are allowed for mailbox validated SMIME certificates
func (l *strictEKUCheck) Execute(c *x509.Certificate) *lint.LintResult {
	hasEmailProtectionEKU := false

	for _, eku := range c.ExtKeyUsage {
		if eku == x509.ExtKeyUsageEmailProtection {
			hasEmailProtectionEKU = true
		} else {
			return &lint.LintResult{Status: lint.Error}
		}
	}

	if hasEmailProtectionEKU {
		return &lint.LintResult{Status: lint.Pass}
	}

	return &lint.LintResult{Status: lint.Error}
}
