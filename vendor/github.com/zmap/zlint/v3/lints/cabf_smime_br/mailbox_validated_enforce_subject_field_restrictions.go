package cabf_smime_br

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

import (
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

// mailboxValidatedEnforceSubjectFieldRestrictions - linter to enforce MAY/SHALL NOT requirements for mailbox validated SMIME certificates
type mailboxValidatedEnforceSubjectFieldRestrictions struct {
	forbiddenSubjectFields map[string]string
	allowedSubjectFields   map[string]string
}

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_mailbox_validated_enforce_subject_field_restrictions",
			Description:   "SMIME certificates complying to mailbox validated profiles MAY only contain commonName, serialNumber or emailAddress attributes in the Subject DN",
			Citation:      "SMIME BRs: 7.1.4.2.3",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: func() lint.CertificateLintInterface {
			return NewMailboxValidatedEnforceSubjectFieldRestrictions()
		},
	})
}

// NewMailboxValidatedEnforceSubjectFieldRestrictions creates a new linter to enforce MAY/SHALL NOT field requirements for mailbox validated SMIME certs
func NewMailboxValidatedEnforceSubjectFieldRestrictions() lint.LintInterface {
	return &mailboxValidatedEnforceSubjectFieldRestrictions{
		forbiddenSubjectFields: map[string]string{
			"0.9.2342.19200300.100.1.25": "subject:domainComponent",
			"1.3.6.1.4.1.311.60.2.1.1":   "subject:jurisdictionLocality",
			"1.3.6.1.4.1.311.60.2.1.2":   "subject:jurisdictionProvince",
			"1.3.6.1.4.1.311.60.2.1.3":   "subject:jurisdictionCountry",
			"2.5.4.4":                    "subject:surname",
			"2.5.4.6":                    "subject:countryName",
			"2.5.4.7":                    "subject:localityName",
			"2.5.4.8":                    "subject:stateOrProvinceName",
			"2.5.4.9":                    "subject:streetAddress",
			"2.5.4.10":                   "subject:organizationName",
			"2.5.4.11":                   "subject:organizationalUnitName",
			"2.5.4.12":                   "subject:title",
			"2.5.4.17":                   "subject:postalCode",
			"2.5.4.42":                   "subject:givenName",
			"2.5.4.65":                   "subject:pseudonym",
			"2.5.4.97":                   "subject:organizationIdentifier",
		},
		allowedSubjectFields: map[string]string{
			"1.2.840.113549.1.9.1": "subject:emailAddress",
			"2.5.4.3":              "subject:commonName",
			"2.5.4.5":              "subject:serialNumber",
		},
	}
}

// CheckApplies returns true if the provided certificate is a subscriber certificate and contains one-or-more of the following
// SMIME BR policy identifiers:
//   - Mailbox Validated Legacy
//   - Mailbox Validated Multipurpose
//   - Mailbox Validated Strict
func (l *mailboxValidatedEnforceSubjectFieldRestrictions) CheckApplies(c *x509.Certificate) bool {
	return util.IsMailboxValidatedCertificate(c) && util.IsSubscriberCert(c)
}

// Execute applies the requirements on what fields are allowed for mailbox validated SMIME certificates
func (l *mailboxValidatedEnforceSubjectFieldRestrictions) Execute(c *x509.Certificate) *lint.LintResult {
	for _, rdnSeq := range c.Subject.OriginalRDNS {
		for _, field := range rdnSeq {
			oidStr := field.Type.String()

			if _, ok := l.allowedSubjectFields[oidStr]; !ok {
				if fieldName, knownField := l.forbiddenSubjectFields[oidStr]; knownField {
					return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("subject DN contains forbidden field: %s (%s)", fieldName, oidStr)}
				}
				return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("subject DN contains forbidden field: %s", oidStr)}
			}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}
