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
	"regexp"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

// Regex to match the start of an organization identifier: 3 character registration scheme identifier and 2 character ISO 3166 country code
var countryRegex = regexp.MustCompile(`^([A-Z]{3})([A-Z]{2})`)

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_registration_scheme_id_matches_subject_country",
			Description:   "The country code used in the Registration Scheme identifier SHALL match that of the subject:countryName in the Certificate as specified in Section 7.1.4.2.2",
			Citation:      "Appendix A.1",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewRegistrationSchemeIDMatchesSubjectCountry,
	})
}

type registrationSchemeIDMatchesSubjectCountry struct{}

// NewRegistrationSchemeIDMatchesSubjectCountry creates a new linter to enforce SHALL requirements for registration scheme identifiers matching subject:countryName
func NewRegistrationSchemeIDMatchesSubjectCountry() lint.CertificateLintInterface {
	return &registrationSchemeIDMatchesSubjectCountry{}
}

// CheckApplies returns true if the provided certificate contains subject:countryName 2 characters in length, a partially valid subject.organizationID and an Organization or Sponsor Validated policy OID
func (l *registrationSchemeIDMatchesSubjectCountry) CheckApplies(c *x509.Certificate) bool {
	if c.Subject.Country == nil {
		return false
	}

	if len(c.Subject.Country[0]) != 2 {
		return false
	}

	orgIDsAreInternational := true
	for _, id := range c.Subject.OrganizationIDs {
		submatches := countryRegex.FindStringSubmatch(id)
		if len(submatches) < 3 {
			return false
		}

		orgIDsAreInternational = orgIDsAreInternational && (submatches[1] == "INT" || submatches[1] == "LEI")
	}

	if orgIDsAreInternational {
		return false
	}

	return util.IsOrganizationValidatedCertificate(c) || util.IsSponsorValidatedCertificate(c)
}

// Execute applies the requirements on matching subject:countryName with registration scheme identifiers
func (l *registrationSchemeIDMatchesSubjectCountry) Execute(c *x509.Certificate) *lint.LintResult {
	country := c.Subject.Country[0]

	for _, id := range c.Subject.OrganizationIDs {
		if err := verifySMIMEOrganizationIdentifierContainsSubjectNameCountry(id, country); err != nil {
			return &lint.LintResult{Status: lint.Error, Details: err.Error()}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}

// verifySMIMEOrganizationIdentifierContainSubjectNameCountry verifies that the country code used in the subject:organizationIdentifier matches subject:countryName
func verifySMIMEOrganizationIdentifierContainsSubjectNameCountry(id string, country string) error {
	submatches := countryRegex.FindStringSubmatch(id)

	if submatches[1] == "INT" || submatches[1] == "LEI" {
		return nil
	}

	// Captures the country code from the organization identifier
	// Note that this raw indexing into the second position is only safe
	// due to a length check done in CheckApplies
	identifierCountry := submatches[2]

	if identifierCountry != country {
		return fmt.Errorf("the country code used in the Registration Scheme identifier SHALL match that of the subject:countryName")
	}

	return nil
}
