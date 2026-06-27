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
	"errors"

	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type unrecommendedQualifier struct{}

type policyInformation struct {
	policyIdentifier      asn1.ObjectIdentifier
	policyQualifiersBytes asn1.RawValue
}

/*******************************************************************
RFC 5280: 4.2.1.4
To promote interoperability, this profile RECOMMENDS that policy
information terms consist of only an OID.  Where an OID alone is
insufficient, this profile strongly recommends that the use of
qualifiers be limited to those identified in this section.  When
qualifiers are used with the special policy anyPolicy, they MUST be
limited to the qualifiers identified in this section.  Only those
qualifiers returned as a result of path validation are considered.
********************************************************************/

func init() {
	lint.RegisterCertificateLint(&lint.CertificateLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_ext_cert_policy_disallowed_any_policy_qualifier",
			Description:   "When qualifiers are used with the special policy anyPolicy, they must be limited to qualifiers identified in this section: (4.2.1.4)",
			Citation:      "RFC 5280: 4.2.1.4",
			Source:        lint.RFC5280,
			EffectiveDate: util.RFC3280Date,
		},
		Lint: NewUnrecommendedQualifier,
	})
}

func NewUnrecommendedQualifier() lint.LintInterface {
	return &unrecommendedQualifier{}
}

func (l *unrecommendedQualifier) CheckApplies(c *x509.Certificate) bool {

	// TODO? extract to util method: HasAnyPolicyOID(c)
	if !util.IsExtInCert(c, util.CertPolicyOID) {
		return false
	}

	for _, policyIds := range c.PolicyIdentifiers {
		if policyIds.Equal(util.AnyPolicyOID) {
			return true
		}
	}
	return false
}

func (l *unrecommendedQualifier) Execute(c *x509.Certificate) *lint.LintResult {

	var err, certificatePolicies = getCertificatePolicies(c)

	if err != nil {
		return &lint.LintResult{Status: lint.Fatal, Details: err.Error()}
	}

	for _, policyInformation := range certificatePolicies {

		if !policyInformation.policyIdentifier.Equal(util.AnyPolicyOID) { // if the policyIdentifier is not anyPolicy do not examine further
			continue
		}

		if len(policyInformation.policyQualifiersBytes.Bytes) == 0 { // this policy information does not have any policyQualifiers
			continue
		}

		var policyQualifiersSeq, policyQualifierInfoSeq asn1.RawValue

		empty, err := asn1.Unmarshal(policyInformation.policyQualifiersBytes.Bytes, &policyQualifiersSeq)

		if err != nil || len(empty) != 0 || policyQualifiersSeq.Class != 0 || policyQualifiersSeq.Tag != 16 || !policyQualifiersSeq.IsCompound {
			return &lint.LintResult{Status: lint.Fatal, Details: "policyExtensions: Could not unmarshal policyQualifiers sequence."}
		}

		//iterate over policyQualifiers ... SEQUENCE SIZE (1..MAX) OF PolicyQualifierInfo OPTIONAL
		for policyQualifierInfoSeqProcessed := false; !policyQualifierInfoSeqProcessed; {
			// these bytes belong to the next PolicyQualifierInfo
			policyQualifiersSeq.Bytes, err = asn1.Unmarshal(policyQualifiersSeq.Bytes, &policyQualifierInfoSeq)
			if err != nil || policyQualifierInfoSeq.Class != 0 || policyQualifierInfoSeq.Tag != 16 || !policyQualifierInfoSeq.IsCompound {
				return &lint.LintResult{Status: lint.Fatal, Details: "policyExtensions: Could not unmarshal policy qualifiers"}
			}
			if len(policyQualifiersSeq.Bytes) == 0 { // no further PolicyQualifierInfo exists
				policyQualifierInfoSeqProcessed = true
			}

			var policyQualifierId asn1.ObjectIdentifier
			_, err = asn1.Unmarshal(policyQualifierInfoSeq.Bytes, &policyQualifierId)
			if err != nil {
				return &lint.LintResult{Status: lint.Fatal, Details: "policyExtensions: Could not unmarshal policyQualifierId."}
			}

			if !policyQualifierId.Equal(util.CpsOID) && !policyQualifierId.Equal(util.UserNoticeOID) {
				return &lint.LintResult{Status: lint.Error}
			}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}

func getCertificatePolicies(c *x509.Certificate) (error, []policyInformation) {

	extVal := util.GetExtFromCert(c, util.CertPolicyOID).Value

	// adjusted code taken from v3/util/oid.go GetMappedPolicies, see comments there
	var certificatePoliciesSeq, policyInformationSeq asn1.RawValue

	empty, err := asn1.Unmarshal(extVal, &certificatePoliciesSeq)

	if err != nil || len(empty) != 0 || certificatePoliciesSeq.Class != 0 || certificatePoliciesSeq.Tag != 16 || !certificatePoliciesSeq.IsCompound {
		return errors.New("policyExtensions: Could not unmarshal certificatePolicies sequence."), nil
	}

	var certificatePolicies []policyInformation

	// iterate over  certificatePolicies ::= SEQUENCE SIZE (1..MAX) OF PolicyInformation
	for policyInformationSeqProcessed := false; !policyInformationSeqProcessed; {

		// these bytes belong to the next PolicyInformation
		certificatePoliciesSeq.Bytes, err = asn1.Unmarshal(certificatePoliciesSeq.Bytes, &policyInformationSeq)
		if err != nil || policyInformationSeq.Class != 0 || policyInformationSeq.Tag != 16 || !policyInformationSeq.IsCompound {
			return errors.New("policyExtensions: Could not unmarshal policyInformation sequence."), nil
		}

		if len(certificatePoliciesSeq.Bytes) == 0 { // no further PolicyInformation exists
			policyInformationSeqProcessed = true
		}

		//PolicyInformation ::= SEQUENCE {
		//	policyIdentifier   CertPolicyId,
		//	policyQualifiers   SEQUENCE SIZE (1..MAX) OF PolicyQualifierInfo OPTIONAL }

		var certPolicyId asn1.ObjectIdentifier
		var policyQualifiers asn1.RawValue
		policyQualifiers.Bytes, err = asn1.Unmarshal(policyInformationSeq.Bytes, &certPolicyId)
		if err != nil {
			return errors.New("policyExtensions: Could not unmarshal certPolicyId."), nil
		}

		information := policyInformation{certPolicyId, policyQualifiers}
		certificatePolicies = append(certificatePolicies, information)
	}
	return nil, certificatePolicies
}
