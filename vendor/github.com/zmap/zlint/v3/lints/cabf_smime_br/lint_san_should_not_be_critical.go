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
	"reflect"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zcrypto/x509/pkix"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "w_san_should_not_be_critical",
			Description:   "subjectAlternativeName SHOULD NOT be marked critical unless the subject field is an empty sequence.",
			Citation:      "7.1.2.3.h",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewSubjectAlternativeNameNotCritical,
	})
}

type SubjectAlternativeNameNotCritical struct{}

func NewSubjectAlternativeNameNotCritical() lint.LintInterface {
	return &SubjectAlternativeNameNotCritical{}
}

func (l *SubjectAlternativeNameNotCritical) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.IsExtInCert(c, util.SubjectAlternateNameOID) && util.IsSMIMEBRCertificate(c)
}

func (l *SubjectAlternativeNameNotCritical) Execute(c *x509.Certificate) *lint.LintResult {
	san := util.GetExtFromCert(c, util.SubjectAlternateNameOID)
	isCritical := san.Critical
	emptySubject := reflect.DeepEqual(c.Subject, pkix.Name{OriginalRDNS: pkix.RDNSequence{}})
	if isCritical && emptySubject {
		// "...unless the subject field is an empty sequence"
		return &lint.LintResult{Status: lint.Pass}
	} else if isCritical && !emptySubject {
		// Critical, but there's a non-empty SAN.
		return &lint.LintResult{
			Status:  lint.Warn,
			Details: "subject is not empty, but subjectAlternativeName is marked critical",
		}
	} else {
		// Not critical, not empty SAN.
		return &lint.LintResult{Status: lint.Pass}
	}
}
