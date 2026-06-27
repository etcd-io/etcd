// (c) Copyright gosec's authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package analyzers

import (
	"golang.org/x/tools/go/analysis"

	"github.com/securego/gosec/v2/taint"
)

// AnalyzerDefinition contains the description of an analyzer and a mechanism to
// create it.
type AnalyzerDefinition struct {
	ID          string
	Description string
	Create      AnalyzerBuilder
}

// AnalyzerBuilder is used to register an analyzer definition with the analyzer
type AnalyzerBuilder func(id string, description string) *analysis.Analyzer

// Taint analysis rule definitions
var (
	SQLInjectionRule = taint.RuleInfo{
		ID:          "G701",
		Description: "SQL injection via string concatenation",
		Severity:    "HIGH",
		CWE:         "CWE-89",
	}

	CommandInjectionRule = taint.RuleInfo{
		ID:          "G702",
		Description: "Command injection via user input",
		Severity:    "CRITICAL",
		CWE:         "CWE-78",
	}

	PathTraversalRule = taint.RuleInfo{
		ID:          "G703",
		Description: "Path traversal via user input",
		Severity:    "HIGH",
		CWE:         "CWE-22",
	}

	SSRFRule = taint.RuleInfo{
		ID:          "G704",
		Description: "SSRF via user-controlled URL",
		Severity:    "HIGH",
		CWE:         "CWE-918",
	}

	XSSRule = taint.RuleInfo{
		ID:          "G705",
		Description: "XSS via unescaped user input",
		Severity:    "MEDIUM",
		CWE:         "CWE-79",
	}

	LogInjectionRule = taint.RuleInfo{
		ID:          "G706",
		Description: "Log injection via user input",
		Severity:    "LOW",
		CWE:         "CWE-117",
	}

	SMTPInjectionRule = taint.RuleInfo{
		ID:          "G707",
		Description: "SMTP command/header injection via user input",
		Severity:    "HIGH",
		CWE:         "CWE-93",
	}

	SSTIRule = taint.RuleInfo{
		ID:          "G708",
		Description: "Server-side template injection via text/template",
		Severity:    "CRITICAL",
		CWE:         "CWE-94",
	}

	UnsafeDeserializationRule = taint.RuleInfo{
		ID:          "G709",
		Description: "Unsafe deserialization of untrusted data",
		Severity:    "HIGH",
		CWE:         "CWE-502",
	}

	OpenRedirectRule = taint.RuleInfo{
		ID:          "G710",
		Description: "Open redirect: user-controlled URL flows into http.Redirect",
		Severity:    "MEDIUM",
		CWE:         "CWE-601",
	}

	FormParsingLimitRule = taint.RuleInfo{
		ID:          "G120",
		Description: "Unbounded multipart form parsing can cause memory exhaustion",
		Severity:    "MEDIUM",
		CWE:         "CWE-400",
	}
)

// AnalyzerList contains a mapping of analyzer ID's to analyzer definitions and a mapping
// of analyzer ID's to whether analyzers are suppressed.
type AnalyzerList struct {
	Analyzers          map[string]AnalyzerDefinition
	AnalyzerSuppressed map[string]bool
}

// AnalyzersInfo returns all the create methods and the analyzer suppressed map for a
// given list
func (al *AnalyzerList) AnalyzersInfo() (map[string]AnalyzerDefinition, map[string]bool) {
	builders := make(map[string]AnalyzerDefinition)
	for _, def := range al.Analyzers {
		builders[def.ID] = def
	}
	return builders, al.AnalyzerSuppressed
}

// AnalyzerFilter can be used to include or exclude an analyzer depending on the return
// value of the function
type AnalyzerFilter func(string) bool

// NewAnalyzerFilter is a closure that will include/exclude the analyzer ID's based on
// the supplied boolean value (false means don't remove, true means exclude).
func NewAnalyzerFilter(action bool, analyzerIDs ...string) AnalyzerFilter {
	analyzerlist := make(map[string]bool)
	for _, analyzer := range analyzerIDs {
		analyzerlist[analyzer] = true
	}
	return func(analyzer string) bool {
		if _, found := analyzerlist[analyzer]; found {
			return action
		}
		return !action
	}
}

var defaultAnalyzers = []AnalyzerDefinition{
	{"G113", "HTTP request smuggling via conflicting headers or bare LF in body parsing", newRequestSmugglingAnalyzer},
	{"G115", "Type conversion which leads to integer overflow", newConversionOverflowAnalyzer},
	{"G118", "Context propagation failure leading to goroutine/resource leaks", newContextPropagationAnalyzer},
	{"G119", "Unsafe redirect policy may propagate sensitive headers", newRedirectHeaderPropagationAnalyzer},
	{"G120", "Unbounded form parsing in HTTP handlers can cause memory exhaustion", newFormParsingLimitAnalyzer},
	{"G121", "Unsafe CrossOriginProtection bypass patterns", newCORSBypassPatternAnalyzer},
	{"G122", "Filesystem TOCTOU race risk in filepath.Walk/WalkDir callbacks", newWalkSymlinkRaceAnalyzer},
	{"G123", "TLS resumption may bypass VerifyPeerCertificate when VerifyConnection is unset", newTLSResumptionVerifyPeerAnalyzer},
	{"G124", "Insecure HTTP cookie configuration missing Secure, HttpOnly, or SameSite attributes", newInsecureCookieAnalyzer},
	{"G602", "Possible slice bounds out of range", newSliceBoundsAnalyzer},
	{"G407", "Use of hardcoded IV/nonce for encryption", newHardCodedNonce},
	{"G408", "Stateful misuse of ssh.PublicKeyCallback leading to auth bypass", newSSHCallbackAnalyzer},
	{"G701", "SQL injection via taint analysis", newSQLInjectionAnalyzer},
	{"G702", "Command injection via taint analysis", newCommandInjectionAnalyzer},
	{"G703", "Path traversal via taint analysis", newPathTraversalAnalyzer},
	{"G704", "SSRF via taint analysis", newSSRFAnalyzer},
	{"G705", "XSS via taint analysis", newXSSAnalyzer},
	{"G706", "Log injection via taint analysis", newLogInjectionAnalyzer},
	{"G707", "SMTP command/header injection via taint analysis", newSMTPInjectionAnalyzer},
	{"G708", "Server-side template injection via taint analysis", newSSTIAnalyzer},
	{"G709", "Unsafe deserialization of untrusted data via taint analysis", newUnsafeDeserializationAnalyzer},
	{"G710", "Open redirect via taint analysis", newOpenRedirectAnalyzer},
}

// Generate the list of analyzers to use
func Generate(trackSuppressions bool, filters ...AnalyzerFilter) *AnalyzerList {
	analyzerMap := make(map[string]AnalyzerDefinition)
	analyzerSuppressedMap := make(map[string]bool)

	for _, analyzer := range defaultAnalyzers {
		analyzerSuppressedMap[analyzer.ID] = false
		addToAnalyzerList := true
		for _, filter := range filters {
			if filter(analyzer.ID) {
				analyzerSuppressedMap[analyzer.ID] = true
				if !trackSuppressions {
					addToAnalyzerList = false
				}
			}
		}
		if addToAnalyzerList {
			analyzerMap[analyzer.ID] = analyzer
		}
	}
	return &AnalyzerList{Analyzers: analyzerMap, AnalyzerSuppressed: analyzerSuppressedMap}
}

// DefaultTaintAnalyzers returns all predefined taint analysis analyzers.
func DefaultTaintAnalyzers() []*analysis.Analyzer {
	sqlConfig := SQLInjection()
	cmdConfig := CommandInjection()
	pathConfig := PathTraversal()
	ssrfConfig := SSRF()
	xssConfig := XSS()
	logConfig := LogInjection()
	smtpConfig := SMTPInjection()
	sstiConfig := SSTI()
	deserConfig := UnsafeDeserialization()
	formConfig := FormParsingLimits()
	openRedirectConfig := OpenRedirect()

	return []*analysis.Analyzer{
		taint.NewGosecAnalyzer(&SQLInjectionRule, &sqlConfig),
		taint.NewGosecAnalyzer(&CommandInjectionRule, &cmdConfig),
		taint.NewGosecAnalyzer(&PathTraversalRule, &pathConfig),
		taint.NewGosecAnalyzer(&SSRFRule, &ssrfConfig),
		taint.NewGosecAnalyzer(&XSSRule, &xssConfig),
		taint.NewGosecAnalyzer(&LogInjectionRule, &logConfig),
		taint.NewGosecAnalyzer(&SMTPInjectionRule, &smtpConfig),
		taint.NewGosecAnalyzer(&SSTIRule, &sstiConfig),
		taint.NewGosecAnalyzer(&UnsafeDeserializationRule, &deserConfig),
		taint.NewGosecAnalyzer(&FormParsingLimitRule, &formConfig),
		taint.NewGosecAnalyzer(&OpenRedirectRule, &openRedirectConfig),
	}
}
