/*
 * ZLint Copyright 2021 Regents of the University of Michigan
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
	"fmt"
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:            "e_underscore_present_with_too_long_validity",
			Description:     "From 2018-12-10 to 2019-04-01, DNSNames may contain underscores if-and-only-if the certificate is valid for less than thirty days.",
			Citation:        "BR 7.1.4.2.1",
			Source:          lint.CABFBaselineRequirements,
			EffectiveDate:   util.CABFBRs_1_6_2_Date,
			IneffectiveDate: util.CABFBRs_1_6_2_UnderscorePermissibilitySunsetDate,
		},
		Lint: func() lint.LintInterface { return &UnderscorePresentWithTooLongValidity{} },
	})
}

type UnderscorePresentWithTooLongValidity struct{}

func (l *UnderscorePresentWithTooLongValidity) CheckApplies(c *x509.Certificate) bool {
	longValidity := util.BeforeOrOn(c.NotBefore.AddDate(0, 0, 30), c.NotAfter)
	return util.IsSubscriberCert(c) && util.DNSNamesExist(c) && longValidity
}

func (l *UnderscorePresentWithTooLongValidity) Execute(c *x509.Certificate) *lint.LintResult {
	for _, dns := range c.DNSNames {
		if strings.Contains(dns, "_") {
			return &lint.LintResult{
				Status: lint.Error,
				Details: fmt.Sprintf(
					"The DNSName '%s' contains an underscore character which is only permissible if the certiticate is valid for less than 30 days (this certificate is valid for %d days)",
					dns,
					c.NotAfter.Sub(c.NotBefore)/util.DurationDay,
				),
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
