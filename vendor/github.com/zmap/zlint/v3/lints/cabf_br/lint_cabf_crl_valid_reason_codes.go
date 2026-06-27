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

type crlHasValidReasonCodes struct{}

func init() {
	lint.RegisterRevocationListLint(&lint.RevocationListLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_cab_crl_has_valid_reason_code",
			Description:   "Only the following CRLReasons MAY be present: 1, 3, 4, 5, 9.",
			Citation:      "BRs: 7.2.2",
			Source:        lint.CABFBaselineRequirements,
			EffectiveDate: util.CABFBRs_1_8_7_Date,
		},
		Lint: NewCrlHasValidReasonCode,
	})
}

func NewCrlHasValidReasonCode() lint.RevocationListLintInterface {
	return &crlHasValidReasonCodes{}
}

func (l *crlHasValidReasonCodes) CheckApplies(c *x509.RevocationList) bool {
	return len(c.RevokedCertificates) > 0
}

var validReasons = map[int]bool{
	1: true,
	3: true,
	4: true,
	5: true,
	9: true,
}

func (l *crlHasValidReasonCodes) Execute(c *x509.RevocationList) *lint.LintResult {
	for _, c := range c.RevokedCertificates {
		if c.ReasonCode == nil {
			continue
		}
		code := *c.ReasonCode
		if code == 0 {
			return &lint.LintResult{Status: lint.Error, Details: "The reason code CRL entry extension SHOULD be absent instead of using the unspecified (0) reasonCode value."}
		}
		if _, ok := validReasons[code]; !ok {
			return &lint.LintResult{Status: lint.Error, Details: "Reason code not included in BR: 7.2.2"}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
