// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package govulncheck contains the JSON output structs for govulncheck.
//
// govulncheck supports streaming JSON by emitting a series of Message
// objects as it analyzes user code and discovers vulnerabilities.
// Streaming JSON is useful for displaying progress in real-time for
// large projects where govulncheck execution might take some time.
//
// govulncheck JSON emits configuration used to perform the analysis,
// a user-friendly message about what is being analyzed, and the
// vulnerability findings. Findings for the same vulnerability can
// can be emitted several times. For instance, govulncheck JSON will
// emit a finding when it sees that a vulnerable module is required
// before proceeding to check if the vulnerability is imported or called.
// Please see documentation on Message and related types for precise
// details on the stream encoding.
//
// There are no guarantees on the order of messages. The pattern of emitted
// messages can change in the future. Clients can follow code in handler.go
// for consuming the streaming JSON programmatically.
package govulncheck

import (
	"time"

	"golang.org/x/vuln/internal/osv"
)

const (
	// ProtocolVersion is the current protocol version this file implements
	ProtocolVersion = "v1.0.0"
)

// Message is an entry in the output stream. It will always have exactly one
// field filled in.
type Message struct {
	Config   *Config   `json:"config,omitempty"`
	Progress *Progress `json:"progress,omitempty"`
	SBOM     *SBOM     `json:"SBOM,omitempty"`
	// OSV is emitted for every vulnerability in the current database
	// that applies to user modules regardless of their version. If a
	// module is being used at a vulnerable version, the corresponding
	// OSV will be referenced in Findings depending on the type of usage
	// and the desired scan level.
	OSV     *osv.Entry `json:"osv,omitempty"`
	Finding *Finding   `json:"finding,omitempty"`
}

// Config must occur as the first message of a stream and informs the client
// about the information used to generate the findings.
// The only required field is the protocol version.
type Config struct {
	// ProtocolVersion specifies the version of the JSON protocol.
	ProtocolVersion string `json:"protocol_version"`

	// ScannerName is the name of the tool, for example, govulncheck.
	//
	// We expect this JSON format to be used by other tools that wrap
	// govulncheck, which will have a different name.
	ScannerName string `json:"scanner_name,omitempty"`

	// ScannerVersion is the version of the tool.
	ScannerVersion string `json:"scanner_version,omitempty"`

	// DB is the database used by the tool, for example,
	// vuln.go.dev.
	DB string `json:"db,omitempty"`

	// LastModified is the last modified time of the data source.
	DBLastModified *time.Time `json:"db_last_modified,omitempty"`

	// GoVersion is the version of Go used for analyzing standard library
	// vulnerabilities.
	GoVersion string `json:"go_version,omitempty"`

	// ScanLevel instructs govulncheck to analyze at a specific level of detail.
	// Valid values include module, package and symbol.
	ScanLevel ScanLevel `json:"scan_level,omitempty"`

	// ScanMode instructs govulncheck how to interpret the input and
	// what to do with it. Valid values are source, binary, query,
	// and extract.
	ScanMode ScanMode `json:"scan_mode,omitempty"`
}

// SBOM contains minimal information about the artifacts govulncheck is scanning.
type SBOM struct {
	// The go version used by govulncheck when scanning, which also defines
	// the version of the standard library used for detecting vulns.
	GoVersion string `json:"go_version,omitempty"`

	// The set of modules included in the scan.
	Modules []*Module `json:"modules,omitempty"`

	// The roots of the scan, as package paths.
	// For binaries, this will be the main package.
	// For source code, this will be the packages matching the provided package patterns.
	Roots []string `json:"roots,omitempty"`
}

type Module struct {
	// The full module path.
	Path string `json:"path,omitempty"`

	// The version of the module.
	Version string `json:"version,omitempty"`
}

// Progress messages are informational only, intended to allow users to monitor
// the progress of a long running scan.
// A stream must remain fully valid and able to be interpreted with all progress
// messages removed.
type Progress struct {
	// A time stamp for the message.
	Timestamp *time.Time `json:"time,omitempty"`

	// Message is the progress message.
	Message string `json:"message,omitempty"`
}

// Finding contains information on a discovered vulnerability. Each vulnerability
// will likely have multiple findings in JSON mode. This is because govulncheck
// emits findings as it does work, and therefore could emit one module level,
// one package level, and potentially multiple symbol level findings depending
// on scan level.
// Multiple symbol level findings can be emitted when multiple symbols of the
// same vuln are called or govulncheck decides to show multiple traces for the
// same symbol.
type Finding struct {
	// OSV is the id of the detected vulnerability.
	OSV string `json:"osv,omitempty"`

	// FixedVersion is the module version where the vulnerability was
	// fixed. This is empty if a fix is not available.
	//
	// If there are multiple fixed versions in the OSV report, this will
	// be the fixed version in the latest range event for the OSV report.
	//
	// For example, if the range events are
	// {introduced: 0, fixed: 1.0.0} and {introduced: 1.1.0}, the fixed version
	// will be empty.
	//
	// For the stdlib, we will show the fixed version closest to the
	// Go version that is used. For example, if a fix is available in 1.17.5 and
	// 1.18.5, and the GOVERSION is 1.17.3, 1.17.5 will be returned as the
	// fixed version.
	FixedVersion string `json:"fixed_version,omitempty"`

	// Trace contains an entry for each frame in the trace.
	//
	// Frames are sorted starting from the imported vulnerable symbol
	// until the entry point. The first frame in Frames should match
	// Symbol.
	//
	// In binary mode, trace will contain a single-frame with no position
	// information.
	//
	// For module level source findings, the trace will contain a single-frame
	// with no symbol, position, or package information. For package level source
	// findings, the trace will contain a single-frame with no symbol or position
	// information.
	Trace []*Frame `json:"trace,omitempty"`
}

// Frame represents an entry in a finding trace.
type Frame struct {
	// Module is the module path of the module containing this symbol.
	//
	// Importable packages in the standard library will have the path "stdlib".
	Module string `json:"module"`

	// Version is the module version from the build graph.
	Version string `json:"version,omitempty"`

	// Package is the import path.
	Package string `json:"package,omitempty"`

	// Function is the function name.
	Function string `json:"function,omitempty"`

	// Receiver is the receiver type if the called symbol is a method.
	//
	// The client can create the final symbol name by
	// prepending Receiver to FuncName.
	Receiver string `json:"receiver,omitempty"`

	// Position describes an arbitrary source position
	// including the file, line, and column location.
	// A Position is valid if the line number is > 0.
	//
	// The filenames are relative to the directory of
	// the enclosing module and always use "/" for
	// portability.
	Position *Position `json:"position,omitempty"`
}

// Position represents arbitrary source position.
type Position struct {
	Filename string `json:"filename,omitempty"` // filename, if any
	Offset   int    `json:"offset"`             // byte offset, starting at 0
	Line     int    `json:"line"`               // line number, starting at 1
	Column   int    `json:"column"`             // column number, starting at 1 (byte count)
}

// ScanLevel represents the detail level at which a scan occurred.
// This can be necessary to correctly interpret the findings, for instance if
// a scan is at symbol level and a finding does not have a symbol it means the
// vulnerability was imported but not called. If the scan however was at
// "package" level, that determination cannot be made.
type ScanLevel string

const (
	ScanLevelModule  = "module"
	ScanLevelPackage = "package"
	ScanLevelSymbol  = "symbol"
)

// WantSymbols can be used to check whether the scan level is one that is able
// to generate symbol-level findings.
func (l ScanLevel) WantSymbols() bool { return l == ScanLevelSymbol }

// WantPackages can be used to check whether the scan level is one that is able
// to generate package-level findings.
func (l ScanLevel) WantPackages() bool { return l == ScanLevelPackage || l == ScanLevelSymbol }

// ScanMode represents the mode in which a scan occurred. This can
// be necessary to correctly to interpret findings. For instance,
// a binary can be checked for vulnerabilities or the user just wants
// to extract minimal data necessary for the vulnerability check.
type ScanMode string

const (
	ScanModeSource  = "source"
	ScanModeBinary  = "binary"
	ScanModeConvert = "convert"
	ScanModeQuery   = "query"
	ScanModeExtract = "extract" // currently, only binary extraction is supported
)
