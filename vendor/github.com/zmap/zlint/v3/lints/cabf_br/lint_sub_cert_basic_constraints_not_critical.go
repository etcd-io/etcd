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
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type subCertBasicConstCrit struct{}

/************************************************
CA/Browser Forum BRs: 7.1.2.7.6 Subscriber Certificate Extensions

| __Extension__                     | __Presence__    | __Critical__ | __Description__ |
| ----                              | -               | -            | ----- |
| `basicConstraints`                | MAY             | Y            | See [Section 7.1.2.7.8](#71278-subscriber-certificate-basic-constraints) |
************************************************/

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_sub_cert_basic_constraints_not_critical",
			Description:   "basicConstraints MAY appear in the certificate, and when it is included MUST be marked as critical",
			Citation:      "CA/Browser Forum BRs: 7.1.2.7.6",
			Source:        lint.CABFBaselineRequirements,
			EffectiveDate: util.SC62EffectiveDate,
		},
		Lint: NewSubCertBasicConstCrit,
	})
}

func NewSubCertBasicConstCrit() lint.LintInterface {
	return &subCertBasicConstCrit{}
}

func (l *subCertBasicConstCrit) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.IsExtInCert(c, util.BasicConstOID)
}

func (l *subCertBasicConstCrit) Execute(c *x509.Certificate) *lint.LintResult {
	if e := util.GetExtFromCert(c, util.BasicConstOID); e != nil {
		if e.Critical {
			return &lint.LintResult{Status: lint.Pass}
		} else {
			return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("Basic Constraints extension is present (%v) and marked as non-critical", e.Id)}
		}
	}
	return &lint.LintResult{Status: lint.Fatal, Details: "Error processing Basic Constraints extension"}
}
