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
			Name:            "e_underscore_permissible_in_dnsname_if_valid_when_replaced",
			Description:     "From December 10th 2018 to April 1st 2019 DNSNames may contain underscores if-and-only-if every label within each DNS name is a valid LDH label after replacing all underscores with hyphens",
			Citation:        "BR 7.1.4.2.1",
			Source:          lint.CABFBaselineRequirements,
			EffectiveDate:   util.CABFBRs_1_6_2_Date,
			IneffectiveDate: util.CABFBRs_1_6_2_UnderscorePermissibilitySunsetDate,
		},
		Lint: func() lint.LintInterface { return &UnderscorePermissibleInDNSNameIfValidWhenReplaced{} },
	})
}

type UnderscorePermissibleInDNSNameIfValidWhenReplaced struct{}

func (l *UnderscorePermissibleInDNSNameIfValidWhenReplaced) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.DNSNamesExist(c)
}

func (l *UnderscorePermissibleInDNSNameIfValidWhenReplaced) Execute(c *x509.Certificate) *lint.LintResult {
	for _, dns := range c.DNSNames {
		for _, label := range strings.Split(dns, ".") {
			if !strings.Contains(label, "_") || label == "*" {
				continue
			}
			replaced := strings.ReplaceAll(label, "_", "-")
			if !util.IsLDHLabel(replaced) {
				return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("When all underscores (_) in %q are replaced with hypens (-) the result is %q which not a valid LDH label", label, replaced)}
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
