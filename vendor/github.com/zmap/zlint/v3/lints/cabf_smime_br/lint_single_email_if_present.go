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
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_single_email_if_present",
			Description:   "If present, the subject:emailAddress SHALL contain a single Mailbox Address",
			Citation:      "7.1.4.2.h",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: func() lint.LintInterface { return &singleEmailIfPresent{} },
	})
}

type singleEmailIfPresent struct{}

func NewSingleEmailIfPresent() lint.LintInterface {
	return &singleEmailIfPresent{}
}

func (l *singleEmailIfPresent) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && c.EmailAddresses != nil && len(c.EmailAddresses) != 0 && util.IsSMIMEBRCertificate(c)
}

func (l *singleEmailIfPresent) Execute(c *x509.Certificate) *lint.LintResult {
	if len(c.EmailAddresses) == 1 {
		return &lint.LintResult{
			Status: lint.Pass,
		}
	} else {
		return &lint.LintResult{
			Status:       lint.Error,
			Details:      fmt.Sprintf("subject:emailAddress was present and contained %d names (%s)", len(c.EmailAddresses), c.EmailAddresses),
			LintMetadata: lint.LintMetadata{},
		}
	}
}
