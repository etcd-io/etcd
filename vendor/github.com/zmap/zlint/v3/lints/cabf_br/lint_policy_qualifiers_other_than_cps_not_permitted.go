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

package cabf_br

import (
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_policy_qualifiers_other_than_cps_not_permitted",
			Description:   "Policy Qualifiers other than id-qt-cps MUST NOT be present for certificates issued on or after September 15, 2023",
			Citation:      "BRs: 7.1.2.7.9",
			Source:        lint.CABFBaselineRequirements,
			EffectiveDate: util.SC62EffectiveDate,
		},
		Lint: NewPolicyQualifiersOtherThanCpsNotPermitted,
	})
}

type PolicyQualifiersOtherThanCpsNotPermitted struct{}

func NewPolicyQualifiersOtherThanCpsNotPermitted() lint.LintInterface {
	return &PolicyQualifiersOtherThanCpsNotPermitted{}
}

func (l *PolicyQualifiersOtherThanCpsNotPermitted) CheckApplies(c *x509.Certificate) bool {

	return util.IsExtInCert(c, util.CertPolicyOID)

}

func (l *PolicyQualifiersOtherThanCpsNotPermitted) Execute(c *x509.Certificate) *lint.LintResult {
	for _, qualifiers := range c.QualifierId {
		for _, qt := range qualifiers {
			if !qt.Equal(util.CpsOID) {
				return &lint.LintResult{Status: lint.Error}
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}

}
