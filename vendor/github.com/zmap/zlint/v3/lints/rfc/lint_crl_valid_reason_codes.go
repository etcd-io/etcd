package rfc

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
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type crlHasValidReasonCode struct{}

/*
***********************************************
RFC 5280: 5.3.1

	CRL issuers are strongly
	  encouraged to include meaningful reason codes in CRL entries;
	  however, the reason code CRL entry extension SHOULD be absent instead
	  of using the unspecified (0) reasonCode value.

***********************************************
*/
func init() {
	lint.RegisterRevocationListLint(&lint.RevocationListLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_crl_has_valid_reason_code",
			Description:   "If a CRL entry has a reason code, it MUST be in RFC5280 section 5.3.1 and SHOULD be absent instead of using unspecified (0)",
			Citation:      "RFC 5280: 5.3.1",
			Source:        lint.RFC5280,
			EffectiveDate: util.RFC5280Date,
		},
		Lint: NewCrlHasValidReasonCode,
	})
}

func NewCrlHasValidReasonCode() lint.RevocationListLintInterface {
	return &crlHasValidReasonCode{}
}

func (l *crlHasValidReasonCode) CheckApplies(c *x509.RevocationList) bool {
	return len(c.RevokedCertificates) > 0
}

func (l *crlHasValidReasonCode) Execute(c *x509.RevocationList) *lint.LintResult {
	for _, c := range c.RevokedCertificates {
		if c.ReasonCode == nil {
			continue
		}
		code := *c.ReasonCode
		if code == 0 {
			return &lint.LintResult{Status: lint.Warn, Details: "The reason code CRL entry extension SHOULD be absent instead of using the unspecified (0) reasonCode value."}
		}
		if code == 7 || code > 10 {
			return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("Reason code, %v, not included in RFC 5280 section 5.3.1", code)}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
