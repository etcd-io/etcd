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
			Name:          "e_subscribers_shall_have_crl_distribution_points",
			Description:   "cRLDistributionPoints SHALL be present.",
			Citation:      "7.1.2.3.b",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewSubscriberCrlDistributionPoints,
	})
}

type SubscriberCrlDistributionPoints struct{}

func NewSubscriberCrlDistributionPoints() lint.LintInterface {
	return &SubscriberCrlDistributionPoints{}
}

func (l *SubscriberCrlDistributionPoints) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.IsSMIMEBRCertificate(c)
}

func (l *SubscriberCrlDistributionPoints) Execute(c *x509.Certificate) *lint.LintResult {
	if len(c.CRLDistributionPoints) == 0 {
		return &lint.LintResult{
			Status:  lint.Error,
			Details: "SMIME certificate contains zero CRL distribution points",
		}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
