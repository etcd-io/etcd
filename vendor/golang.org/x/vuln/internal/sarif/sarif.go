// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sarif defines Static Analysis Results Interchange Format
// (SARIF) types supported by govulncheck.
//
// The implementation covers the subset of the specification available
// at https://www.oasis-open.org/committees/tc_home.php?wg_abbrev=sarif.
//
// The sarif encoding models govulncheck findings as Results. Each
// Result encodes findings for a unique OSV entry at the most precise
// detected level only. CodeFlows summarize call stacks, similar to govulncheck
// textual output, while Stacks contain call stack information verbatim.
//
// The result Levels are defined by the govulncheck.ScanLevel and the most
// precise level at which the finding was detected. Result error is produced
// when the finding level matches the user desired level of scan precision;
// all other finding levels are then classified as progressively weaker.
// For instance, if the user specified symbol scan level and govulncheck
// detected a use of a vulnerable symbol, then the Result will have error
// Level. If the symbol was not used but its package was imported, then the
// Result Level is warning, and so on.
//
// Each Result is attached to the first line of the go.mod file. Other
// ArtifactLocations are paths relative to their enclosing modules.
// Similar to JSON output format, this makes govulncheck sarif locations
// portable.
//
// The relative paths in PhysicalLocations also come with a URIBaseID offset.
// Paths for the source module analyzed, the Go standard library, and third-party
// dependencies are relative to %SRCROOT%, %GOROOT%, and %GOMODCACHE% offsets,
// resp. We note that the URIBaseID offsets are not explicitly defined in
// the sarif output. It is the clients responsibility to set them to resolve
// paths at their local machines.
//
// All paths use "/" delimiter for portability.
//
// Properties field of a Tool.Driver is a govulncheck.Config used for the
// invocation of govulncheck producing the Results. Properties field of
// a Rule contains information on CVE and GHSA aliases for the corresponding
// rule OSV. Clients can use this information to, say, suppress and filter
// vulnerabilities.
//
// Please see the definition of types below for more information.
package sarif

import "golang.org/x/vuln/internal/govulncheck"

// Log is the top-level SARIF object encoded in UTF-8.
type Log struct {
	// Version should always be "2.1.0"
	Version string `json:"version,omitempty"`

	// Schema should always be "https://json.schemastore.org/sarif-2.1.0.json"
	Schema string `json:"$schema,omitempty"`

	// Runs describes executions of static analysis tools. For govulncheck,
	// there will be only one run object.
	Runs []Run `json:"runs,omitempty"`
}

// Run summarizes results of a single invocation of a static analysis tool,
// in this case govulncheck.
type Run struct {
	Tool Tool `json:"tool,omitempty"`
	// Results contain govulncheck findings. There should be exactly one
	// Result per a detected use of an OSV.
	Results []Result `json:"results"`
}

// Tool captures information about govulncheck analysis that was run.
type Tool struct {
	Driver Driver `json:"driver,omitempty"`
}

// Driver provides details about the govulncheck binary being executed.
type Driver struct {
	// Name is "govulncheck"
	Name string `json:"name,omitempty"`
	// Version is the govulncheck version
	Version string `json:"semanticVersion,omitempty"`
	// InformationURI points to the description of govulncheck tool
	InformationURI string `json:"informationUri,omitempty"`
	// Properties are govulncheck run metadata, such as vuln db, Go version, etc.
	Properties govulncheck.Config `json:"properties,omitempty"`

	Rules []Rule `json:"rules"`
}

// Rule corresponds to the static analysis rule/analyzer that
// produces findings. For govulncheck, rules are OSVs.
type Rule struct {
	// ID is OSV.ID
	ID               string      `json:"id,omitempty"`
	ShortDescription Description `json:"shortDescription,omitempty"`
	FullDescription  Description `json:"fullDescription,omitempty"`
	Help             Description `json:"help,omitempty"`
	HelpURI          string      `json:"helpUri,omitempty"`
	// Properties contain OSV.Aliases (CVEs and GHSAs) as tags.
	// Consumers of govulncheck SARIF can use these tags to filter
	// results.
	Properties RuleTags `json:"properties,omitempty"`
}

// RuleTags defines properties.tags.
type RuleTags struct {
	Tags []string `json:"tags"`
}

// Description is a text in its raw or markdown form.
type Description struct {
	Text     string `json:"text,omitempty"`
	Markdown string `json:"markdown,omitempty"`
}

// Result is a set of govulncheck findings for an OSV. For call stack
// mode, it will contain call stacks for the OSV. There is exactly
// one Result per detected OSV. Only findings at the most precise
// detected level appear in the Result. For instance, if there are
// symbol findings for an OSV, those findings will be in the Result,
// but not the package and module level findings for the same OSV.
type Result struct {
	// RuleID is the Rule.ID/OSV producing the finding.
	RuleID string `json:"ruleId,omitempty"`
	// Level is one of "error", "warning", and "note".
	Level string `json:"level,omitempty"`
	// Message explains the overall findings.
	Message Description `json:"message,omitempty"`
	// Locations to which the findings are associated. Always
	// a single location pointing to the first line of the go.mod
	// file. The path to the file is "go.mod".
	Locations []Location `json:"locations,omitempty"`
	// CodeFlows summarize call stacks produced by govulncheck.
	CodeFlows []CodeFlow `json:"codeFlows,omitempty"`
	// Stacks encode call stacks produced by govulncheck.
	Stacks []Stack `json:"stacks,omitempty"`
}

// CodeFlow summarizes a detected offending flow of information in terms of
// code locations. More precisely, it can contain several related information
// flows, keeping them together. In govulncheck, those can be all call stacks
// for, say, a particular symbol or package.
type CodeFlow struct {
	// ThreadFlows is effectively a set of related information flows.
	ThreadFlows []ThreadFlow `json:"threadFlows"`
	Message     Description  `json:"message,omitempty"`
}

// ThreadFlow encodes an information flow as a sequence of locations.
// For govulncheck, it can encode a call stack.
type ThreadFlow struct {
	Locations []ThreadFlowLocation `json:"locations,omitempty"`
}

type ThreadFlowLocation struct {
	// Module is module information in the form <module-path>@<version>.
	// <version> can be empty when the module version is not known as
	// with, say, the source module analyzed.
	Module string `json:"module,omitempty"`
	// Location also contains a Message field.
	Location Location `json:"location,omitempty"`
}

// Stack is a sequence of frames and can encode a govulncheck call stack.
type Stack struct {
	Message Description `json:"message,omitempty"`
	Frames  []Frame     `json:"frames"`
}

// Frame is effectively a module location. It can also contain thread and
// parameter info, but those are not needed for govulncheck.
type Frame struct {
	// Module is module information in the form <module-path>@<version>.
	// <version> can be empty when the module version is not known as
	// with, say, the source module analyzed.
	Module   string   `json:"module,omitempty"`
	Location Location `json:"location,omitempty"`
}

// Location is currently a physical location annotated with a message.
type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation,omitempty"`
	Message          Description      `json:"message,omitempty"`
}

type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation,omitempty"`
	Region           Region           `json:"region,omitempty"`
}

const (
	SrcRootID    = "%SRCROOT%"
	GoRootID     = "%GOROOT%"
	GoModCacheID = "%GOMODCACHE%"
)

// ArtifactLocation is a path to an offending file.
type ArtifactLocation struct {
	// URI is a path relative to URIBaseID.
	URI string `json:"uri,omitempty"`
	// URIBaseID is offset for URI, one of %SRCROOT%, %GOROOT%,
	// and %GOMODCACHE%.
	URIBaseID string `json:"uriBaseId,omitempty"`
}

// Region is a target region within a file.
type Region struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}
