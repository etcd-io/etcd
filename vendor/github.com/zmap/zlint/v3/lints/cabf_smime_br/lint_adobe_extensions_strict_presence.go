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
			Name:          "e_adobe_extensions_strict_presence",
			Description:   "Adobe Time‐stamp X509 extension (1.2.840.113583.1.1.9.1) and the Adobe ArchiveRevInfo extension (1.2.840.113583.1.1.9.2) are prohibited for strict SMIME certificates",
			Citation:      "7.1.2.3.m",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewAdobeExtensionsStrictPresence,
	})
}

type adobeExtensionsStrictPresence struct{}

// NewAdobeExtensionsStrictPresence creates a new linter to enforce adobe x509 extensions requirements for strict SMIME certs
func NewAdobeExtensionsStrictPresence() lint.CertificateLintInterface {
	return &adobeExtensionsStrictPresence{}
}

// CheckApplies returns true if for any subscriber certificate the certificate's policies assert that it conforms to the strict policy requirements defined in the SMIME BRs
func (l *adobeExtensionsStrictPresence) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.IsStrictSMIMECertificate(c)
}

// Execute applies the requirements of adobe x509 extensions not being allowed for strict SMIME certificates
func (l *adobeExtensionsStrictPresence) Execute(c *x509.Certificate) *lint.LintResult {
	if hasAdobeX509Extensions(c) {
		return &lint.LintResult{Status: lint.Error}
	}

	return &lint.LintResult{Status: lint.Pass}
}

func hasAdobeX509Extensions(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.AdobeTimeStampOID) || util.IsExtInCert(c, util.AdobeArchiveRevInfoOID)
}
