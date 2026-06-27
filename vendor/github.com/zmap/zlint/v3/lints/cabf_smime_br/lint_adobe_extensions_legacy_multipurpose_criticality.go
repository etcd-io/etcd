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
			Name:          "e_adobe_extensions_legacy_multipurpose_criticality",
			Description:   "If present, Adobe Time‐stamp X509 extension (1.2.840.113583.1.1.9.1) or the Adobe ArchiveRevInfo extension (1.2.840.113583.1.1.9.2) SHALL NOT be marked as critical for multipurpose/legacy SMIME certificates",
			Citation:      "7.1.2.3.m",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewAdobeExtensionsLegacyMultipurposeCriticality,
	})
}

type adobeExtensionsLegacyMultipurposeCriticality struct{}

// NewAdobeExtensionsLegacyMultipurposeCriticality creates a new linter to enforce adobe x509 extensions requirements for multipurpose or legacy SMIME certs
func NewAdobeExtensionsLegacyMultipurposeCriticality() lint.CertificateLintInterface {
	return &adobeExtensionsLegacyMultipurposeCriticality{}
}

// CheckApplies returns true if for any subscriber certificate the certificate's policies assert that it conforms to the multipurpose or legacy policy requirements defined in the SMIME BRs
// and the certificate contains one of the adobe x509 extensions
func (l *adobeExtensionsLegacyMultipurposeCriticality) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && (util.IsLegacySMIMECertificate(c) || util.IsMultipurposeSMIMECertificate(c)) && hasAdobeX509Extensions(c)
}

// Execute applies the requirements of adobe x509 extensions not being marked as critical, if present, for multipurpose or legacy SMIME certificates
func (l *adobeExtensionsLegacyMultipurposeCriticality) Execute(c *x509.Certificate) *lint.LintResult {
	adobeTimeStampExt := util.GetExtFromCert(c, util.AdobeTimeStampOID)
	if adobeTimeStampExt != nil && adobeTimeStampExt.Critical {
		return &lint.LintResult{Status: lint.Error}
	}

	adobeArchRevInfoExt := util.GetExtFromCert(c, util.AdobeArchiveRevInfoOID)
	if adobeArchRevInfoExt != nil && adobeArchRevInfoExt.Critical {
		return &lint.LintResult{Status: lint.Error}
	}

	return &lint.LintResult{Status: lint.Pass}
}
