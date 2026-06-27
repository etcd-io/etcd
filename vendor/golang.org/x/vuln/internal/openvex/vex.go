// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vex defines the Vulnerability EXchange Format (VEX) types
// supported by govulncheck.
//
// These types match the OpenVEX standard. See https://github.com/openvex for
// more information on VEX and OpenVEX.
//
// This is intended to be the minimimal amount of information required to output
// a complete VEX document according to the specification.
package openvex

import "time"

const (
	ContextURI = "https://openvex.dev/ns/v0.2.0"
	Tooling    = "https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck"
	Impact     = "Govulncheck determined that the vulnerable code isn't called"

	DefaultAuthor = "Unknown Author"
	DefaultPID    = "Unknown Product"

	// The following are defined by the VEX standard.
	StatusAffected    = "affected"
	StatusNotAffected = "not_affected"

	// The following are defined by the VEX standard.
	JustificationNotExecuted = "vulnerable_code_not_in_execute_path"
	JustificationNotPresent  = "vulnerable_code_not_present"
)

// Document is the top-level struct for a VEX document.
type Document struct {
	// Context is an IRI pointing to the version of openVEX being used by the doc
	// For govulncheck, it will always be https://openvex.dev/ns/v0.2.0
	Context string `json:"@context,omitempty"`

	// ID is the identifying string for the VEX document.
	// govulncheck/vex-[content-based-hash]
	ID string `json:"@id,omitempty"`

	// Author is the identifier for the author of the VEX statement.
	// Govulncheck will leave this field default (Unknown author) to be filled in by the user.
	Author string `json:"author,omitempty"`

	// Timestamp defines the time at which the document was issued.
	Timestamp time.Time `json:"timestamp,omitempty"`

	// Version is the document version. For govulncheck's output, this will always be 1.
	Version int `json:"version,omitempty"`

	// Tooling expresses how the VEX document and contained VEX statements were
	// generated. In this case, it will always be:
	// "https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck"
	Tooling string `json:"tooling,omitempty"`

	// Statements are all statements for a given govulncheck output.
	// Each OSV emitted by govulncheck will have a corresponding statement.
	Statements []Statement `json:"statements,omitempty"`
}

// Statement conveys a single status for a single vulnerability for one or more products.
type Statement struct {
	// Vulnerability is the vuln being referenced by the statement.
	Vulnerability Vulnerability `json:"vulnerability,omitempty"`

	// Products are the products associated with the given vulnerability in the statement.
	Products []Product `json:"products,omitempty"`

	// The status of the vulnerability. Will be either not_affected or affected for govulncheck.
	Status string `json:"status,omitempty"`

	// If the status is not_affected, this must be filled. The official VEX justification that
	// best matches govulncheck's vuln filtering is "vulnerable_code_not_in_execute_path"
	Justification string `json:"justification,omitempty"`

	// If the status is not_affected, this must be filled. For govulncheck, this will always be:
	// "Govulncheck determined that the vulnerable code isn't called"
	ImpactStatement string `json:"impact_statement,omitempty"`
}

// Vulnerability captures a vulnerability and its identifiers/aliases.
type Vulnerability struct {
	// ID is a URI that in govulncheck's case points to the govulndb link for the vulnerability.
	// I.E. https://pkg.go.dev/vuln/GO-2024-2497
	ID string `json:"@id,omitempty"`

	// Name is the main identifier for the vulnerability (GO-YYYY-XXXX)
	Name string `json:"name,omitempty"`

	// Description is a short text description of the vulnerability.
	// It will be populated from the 'summary' field of the vuln's OSV if it exists,
	// and the 'description' field of the osv if a summary isn't present.
	Description string `json:"description,omitempty"`

	// Aliases a list of identifiers that other systems are using to track the vulnerability.
	// I.E. GHSA or CVE ids.
	Aliases []string `json:"aliases,omitempty"`
}

// Product identifies the products associated with the given vuln.
type Product struct {
	// The main product ID will remian default for now.
	Component
	// The subcomponent ID will be a PURL to the vulnerable dependency.
	Subcomponents []Component `json:"subcomponents,omitempty"`
}

type Component struct {
	ID string `json:"@id,omitempty"`
}
