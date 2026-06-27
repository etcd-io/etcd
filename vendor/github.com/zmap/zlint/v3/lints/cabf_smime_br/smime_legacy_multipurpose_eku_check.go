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

// shallHaveCrlDistributionPoints - linter to enforce requirement that SMIME certificates SHALL contain emailProtecton EKU
type legacyMultipurposeEKUCheck struct {
}

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_smime_legacy_multipurpose_eku_check",
			Description:   "Strict/Multipurpose and Legacy: id-kp-emailProtection SHALL be present. Other values MAY be present.  The values id-kp-serverAuth, id-kp-codeSigning, id-kp-timeStamping, and anyExtendedKeyUsage values SHALL NOT be present.",
			Citation:      "SMIME BRs: 7.1.2.3.f",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewLegacyMultipurposeEKUCheck,
	})
}

// NewShallHaveCrlDistributionPoints creates a new linter to enforce MAY/SHALL NOT field requirements for mailbox validated SMIME certs
func NewLegacyMultipurposeEKUCheck() lint.CertificateLintInterface {
	return &legacyMultipurposeEKUCheck{}
}

// CheckApplies returns true if the provided certificate contains one-or-more of the following SMIME BR policy identifiers:
//   - Mailbox Validated Legacy
//   - Mailbox Validated Multipurpose
//   - Organization Validated Legacy
//   - Organization Validated Multipurpose
//   - Sponsor Validated Legacy
//   - Sponsor Validated Multipurpose
//   - Individual Validated Legacy
//   - Individual Validated Multipurpose
func (l *legacyMultipurposeEKUCheck) CheckApplies(c *x509.Certificate) bool {
	return (util.IsLegacySMIMECertificate(c) || util.IsMultipurposeSMIMECertificate(c)) && util.IsSubscriberCert(c)
}

// Execute applies the requirements on what fields are allowed for mailbox validated SMIME certificates
func (l *legacyMultipurposeEKUCheck) Execute(c *x509.Certificate) *lint.LintResult {
	hasEmailProtectionEKU := false
	ekusOK := true

	for _, eku := range c.ExtKeyUsage {
		if eku == x509.ExtKeyUsageEmailProtection {
			hasEmailProtectionEKU = true
		} else if eku == x509.ExtKeyUsageServerAuth || eku == x509.ExtKeyUsageCodeSigning || eku == x509.ExtKeyUsageTimeStamping || eku == x509.ExtKeyUsageAny {
			ekusOK = false
		}
	}

	if !hasEmailProtectionEKU {
		return &lint.LintResult{Status: lint.Error, Details: "id-kp-emailProtection SHALL be present"}
	}

	if !ekusOK {
		return &lint.LintResult{Status: lint.Error, Details: "id-kp-serverAuth, id-kp-codeSigning, id-kp-timeStamping, and anyExtendedKeyUsage values SHALL NOT be present"}
	}

	return &lint.LintResult{Status: lint.Pass}
}
