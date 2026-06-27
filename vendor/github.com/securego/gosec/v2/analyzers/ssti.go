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

// SSTI returns a configuration for detecting Server-Side Template Injection
// vulnerabilities via text/template.
//
// The text/template package performs NO auto-escaping and allows calling any
// exported method on the data object passed to Execute. When user-controlled
// input flows into Template.Parse, an attacker can invoke arbitrary methods,
// read files, or achieve remote code execution depending on available gadgets.
//
// Even when the template string is static, rendering user data through
// text/template into an HTTP response produces unescaped HTML, enabling XSS.
func SSTI() taint.Config {
	return taint.Config{
		Sources: []taint.Source{
			// Type sources: tainted when received as parameters
			{Package: "net/http", Name: "Request", Pointer: true},
			{Package: "net/url", Name: "Values"},

			// Function sources
			{Package: "os", Name: "Args", IsFunc: true},
			{Package: "os", Name: "Getenv", IsFunc: true},

			// I/O sources
			{Package: "bufio", Name: "Reader", Pointer: true},
			{Package: "bufio", Name: "Scanner", Pointer: true},
		},
		Sinks: []taint.Sink{
			// CRITICAL: user input flows into the template string itself.
			// Template.Parse takes a single string argument (the template text).
			{Package: "text/template", Receiver: "Template", Method: "Parse", Pointer: true, CheckArgs: []int{1}},

			// text/template.Must wraps Parse; arg[0] is the (*Template, error) pair
			// but in practice the taint flows through the Parse call above.

			// HIGH: text/template.Execute writes unescaped output to an HTTP response.
			// Guard: only flag when the writer (arg 1) implements net/http.ResponseWriter.
			{
				Package:       "text/template",
				Receiver:      "Template",
				Method:        "Execute",
				Pointer:       true,
				CheckArgs:     []int{2},
				ArgTypeGuards: map[int]string{1: "net/http.ResponseWriter"},
			},
			{
				Package:       "text/template",
				Receiver:      "Template",
				Method:        "ExecuteTemplate",
				Pointer:       true,
				CheckArgs:     []int{3},
				ArgTypeGuards: map[int]string{1: "net/http.ResponseWriter"},
			},
		},
		Sanitizers: []taint.Sanitizer{
			// HTML escaping neutralizes both SSTI template directives and XSS payloads
			{Package: "html", Method: "EscapeString"},
			{Package: "html/template", Method: "HTMLEscapeString"},
			{Package: "html/template", Method: "JSEscapeString"},
			{Package: "net/url", Method: "QueryEscape"},
			{Package: "net/url", Method: "PathEscape"},

			// Numeric conversions produce safe output
			{Package: "strconv", Method: "Atoi"},
			{Package: "strconv", Method: "Itoa"},
			{Package: "strconv", Method: "ParseInt"},
			{Package: "strconv", Method: "ParseUint"},
			{Package: "strconv", Method: "ParseFloat"},
			{Package: "strconv", Method: "FormatInt"},
			{Package: "strconv", Method: "FormatUint"},
			{Package: "strconv", Method: "FormatFloat"},
		},
	}
}

// newSSTIAnalyzer creates an analyzer for detecting Server-Side Template
// Injection vulnerabilities via taint analysis (G708).
func newSSTIAnalyzer(id string, description string) *analysis.Analyzer {
	config := SSTI()
	rule := SSTIRule
	rule.ID = id
	rule.Description = description
	return taint.NewGosecAnalyzer(&rule, &config)
}
