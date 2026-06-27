package cabf_smime_br

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
	"net/url"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type smimeLegacyAIAHasOneHTTP struct{}

/************************************************************************
BRs: 7.1.2.3c
CA Certificate Authority Information Access
The authorityInformationAccess extension MAY contain one or more accessMethod
values for each of the following types:

id-ad-ocsp        specifies the URI of the Issuing CA's OCSP responder.
id-ad-caIssuers   specifies the URI of the Issuing CA's Certificate.

For Legacy: When provided, at least one accessMethod SHALL have the URI scheme HTTP. Other schemes (LDAP, FTP, ...) MAY be present.
*************************************************************************/

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_smime_legacy_aia_shall_have_one_http",
			Description:   "SMIME Legacy certificates authorityInformationAccess When provided, at least one accessMethod SHALL have the URI scheme HTTP. Other schemes (LDAP, FTP, ...) MAY be present.",
			Citation:      "BRs: 7.1.2.3c",
			Source:        lint.CABFSMIMEBaselineRequirements,
			EffectiveDate: util.CABF_SMIME_BRs_1_0_0_Date,
		},
		Lint: NewSMIMELegacyAIAHasOneHTTP,
	})
}

func NewSMIMELegacyAIAHasOneHTTP() lint.LintInterface {
	return &smimeLegacyAIAHasOneHTTP{}
}

func (l *smimeLegacyAIAHasOneHTTP) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.IsExtInCert(c, util.AiaOID) && util.IsLegacySMIMECertificate(c)
}

func (l *smimeLegacyAIAHasOneHTTP) Execute(c *x509.Certificate) *lint.LintResult {
	atLeastOneHttp := false
	for _, u := range c.OCSPServer {
		purl, err := url.Parse(u)
		if err != nil {
			return &lint.LintResult{Status: lint.Error}
		}
		if purl.Scheme == "http" {
			atLeastOneHttp = true
		}
	}
	if !atLeastOneHttp && len(c.OCSPServer) != 0 {
		return &lint.LintResult{Status: lint.Error, Details: "at least one id-ad-ocsp accessMethod MUST have the URI scheme HTTP"}
	}

	atLeastOneHttp = false
	for _, u := range c.IssuingCertificateURL {
		purl, err := url.Parse(u)
		if err != nil {
			return &lint.LintResult{Status: lint.Error}
		}
		if purl.Scheme == "http" {
			atLeastOneHttp = true
		}
	}
	if !atLeastOneHttp && len(c.IssuingCertificateURL) != 0 {
		return &lint.LintResult{Status: lint.Error, Details: "at least one id-ad-caIssuers accessMethod MUST have the URI scheme HTTP"}
	}

	return &lint.LintResult{Status: lint.Pass}
}
