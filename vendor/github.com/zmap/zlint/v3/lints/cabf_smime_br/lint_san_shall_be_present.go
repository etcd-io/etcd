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
			Name:          "e_san_shall_be_present",
			Description:   "Subject alternative name SHALL be present",
			Citation:      "7.1.2.3.h",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewSubjectAlternativeNameShallBePresent,
	})
}

type subjectAlternativeNameShallBePresent struct{}

func NewSubjectAlternativeNameShallBePresent() lint.LintInterface {
	return &subjectAlternativeNameShallBePresent{}
}

func (l *subjectAlternativeNameShallBePresent) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.IsSMIMEBRCertificate(c)
}

func (l *subjectAlternativeNameShallBePresent) Execute(c *x509.Certificate) *lint.LintResult {
	if !util.IsExtInCert(c, util.SubjectAlternateNameOID) {
		return &lint.LintResult{
			Status:  lint.Error,
			Details: "SMIME certificate does not have a subject alternative name extension",
		}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
