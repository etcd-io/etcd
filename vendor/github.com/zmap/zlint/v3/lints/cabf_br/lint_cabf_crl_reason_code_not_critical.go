package cabf_br

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

type crlReasonCodeNotCritical struct{}

func init() {
	lint.RegisterRevocationListLint(&lint.RevocationListLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_cab_crl_reason_code_not_critical",
			Description:   "If present, CRL Reason Code extension MUST NOT be marked critical.",
			Citation:      "BRs: 7.2.2",
			Source:        lint.CABFBaselineRequirements,
			EffectiveDate: util.CABEffectiveDate,
		},
		Lint: NewCrlReasonCodeNotCritical,
	})
}

func NewCrlReasonCodeNotCritical() lint.RevocationListLintInterface {
	return &crlReasonCodeNotCritical{}
}

func (l *crlReasonCodeNotCritical) CheckApplies(c *x509.RevocationList) bool {
	return len(c.RevokedCertificates) > 0
}

func (l *crlReasonCodeNotCritical) Execute(c *x509.RevocationList) *lint.LintResult {
	for _, c := range c.RevokedCertificates {
		if c.ReasonCode == nil {
			continue
		}
		for _, ext := range c.Extensions {
			if ext.Id.Equal(util.ReasonCodeOID) {
				if ext.Critical {
					return &lint.LintResult{Status: lint.Error, Details: "CRL Reason Code extension MUST NOT be marked as critical."}
				}
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
