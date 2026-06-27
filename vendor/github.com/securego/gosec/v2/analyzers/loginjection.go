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

// LogInjection returns a configuration for detecting log injection vulnerabilities.
func LogInjection() taint.Config {
	return taint.Config{
		Sources: []taint.Source{
			// Type sources: tainted when received as parameters
			{Package: "net/http", Name: "Request", Pointer: true},
			{Package: "net/url", Name: "URL", Pointer: true},

			// Function sources
			{Package: "os", Name: "Args", IsFunc: true},
			{Package: "os", Name: "Getenv", IsFunc: true},

			// I/O sources
			{Package: "bufio", Name: "Reader", Pointer: true},
			{Package: "bufio", Name: "Scanner", Pointer: true},
		},
		Sinks: []taint.Sink{
			{Package: "log", Method: "Print"},
			{Package: "log", Method: "Printf"},
			{Package: "log", Method: "Println"},
			{Package: "log", Method: "Fatal"},
			{Package: "log", Method: "Fatalf"},
			{Package: "log", Method: "Fatalln"},
			{Package: "log", Method: "Panic"},
			{Package: "log", Method: "Panicf"},
			{Package: "log", Method: "Panicln"},
			// log/slog structured logging functions have the signature:
			//   func Warn(msg string, args ...any)
			// The variadic `args` are key-value attribute pairs whose values are
			// automatically escaped by both TextHandler (JSON-quoted) and JSONHandler
			// (JSON-encoded), making them safe against log injection.
			// Only the `msg` argument (args[0]) is a real injection vector because
			// TextHandler writes it verbatim without quoting.
			// CheckArgs: []int{0} scopes the taint check to the message only,
			// preventing false positives on: slog.Warn("msg", "key", taintedVal)
			{Package: "log/slog", Method: "Info", CheckArgs: []int{0}},
			{Package: "log/slog", Method: "Warn", CheckArgs: []int{0}},
			{Package: "log/slog", Method: "Error", CheckArgs: []int{0}},
			{Package: "log/slog", Method: "Debug", CheckArgs: []int{0}},
		},
		Sanitizers: []taint.Sanitizer{
			// strings.ReplaceAll can strip newlines/CRLF for log injection
			{Package: "strings", Method: "ReplaceAll"},
			// strconv.Quote safely quotes a string (escapes special chars)
			{Package: "strconv", Method: "Quote"},
			// url.QueryEscape encodes special characters
			{Package: "net/url", Method: "QueryEscape"},

			// JSON encoding escapes all special characters including newlines,
			// producing structurally safe output for log entries.
			{Package: "encoding/json", Method: "Marshal"},
			{Package: "encoding/json", Method: "MarshalIndent"},

			// Numeric conversions produce strings that cannot contain
			// log injection characters (newlines, carriage returns).
			{Package: "strconv", Method: "Atoi"},
			{Package: "strconv", Method: "Itoa"},
			{Package: "strconv", Method: "ParseInt"},
			{Package: "strconv", Method: "ParseUint"},
			{Package: "strconv", Method: "ParseFloat"},
			{Package: "strconv", Method: "FormatInt"},
			{Package: "strconv", Method: "FormatFloat"},
		},
	}
}

// newLogInjectionAnalyzer creates an analyzer for detecting log injection vulnerabilities
// via taint analysis (G706)
func newLogInjectionAnalyzer(id string, description string) *analysis.Analyzer {
	config := LogInjection()
	rule := LogInjectionRule
	rule.ID = id
	rule.Description = description
	return taint.NewGosecAnalyzer(&rule, &config)
}
