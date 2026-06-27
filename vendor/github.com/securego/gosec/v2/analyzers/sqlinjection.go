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

// SQLInjection returns a configuration for detecting SQL injection vulnerabilities.
func SQLInjection() taint.Config {
	return taint.Config{
		Sources: []taint.Source{
			// Type sources: tainted when received as parameters
			{Package: "net/http", Name: "Request", Pointer: true},
			{Package: "net/url", Name: "URL", Pointer: true},
			{Package: "net/url", Name: "Values"},
			{Package: "bufio", Name: "Reader", Pointer: true},
			{Package: "bufio", Name: "Scanner", Pointer: true},

			// Function sources
			{Package: "os", Name: "Args", IsFunc: true},
			{Package: "os", Name: "Getenv", IsFunc: true},
		},
		Sinks: []taint.Sink{
			// For SQL methods, Args[0] is receiver, Args[1] is query string
			// Only check query string argument; prepared statement params are safe
			{Package: "database/sql", Receiver: "DB", Method: "Query", Pointer: true, CheckArgs: []int{1}},
			{Package: "database/sql", Receiver: "DB", Method: "QueryContext", Pointer: true, CheckArgs: []int{2}},
			{Package: "database/sql", Receiver: "DB", Method: "QueryRow", Pointer: true, CheckArgs: []int{1}},
			{Package: "database/sql", Receiver: "DB", Method: "QueryRowContext", Pointer: true, CheckArgs: []int{2}},
			{Package: "database/sql", Receiver: "DB", Method: "Exec", Pointer: true, CheckArgs: []int{1}},
			{Package: "database/sql", Receiver: "DB", Method: "ExecContext", Pointer: true, CheckArgs: []int{2}},
			{Package: "database/sql", Receiver: "DB", Method: "Prepare", Pointer: true, CheckArgs: []int{1}},
			{Package: "database/sql", Receiver: "DB", Method: "PrepareContext", Pointer: true, CheckArgs: []int{2}},
			{Package: "database/sql", Receiver: "Tx", Method: "Query", Pointer: true, CheckArgs: []int{1}},
			{Package: "database/sql", Receiver: "Tx", Method: "QueryContext", Pointer: true, CheckArgs: []int{2}},
			{Package: "database/sql", Receiver: "Tx", Method: "QueryRow", Pointer: true, CheckArgs: []int{1}},
			{Package: "database/sql", Receiver: "Tx", Method: "QueryRowContext", Pointer: true, CheckArgs: []int{2}},
			{Package: "database/sql", Receiver: "Tx", Method: "Exec", Pointer: true, CheckArgs: []int{1}},
			{Package: "database/sql", Receiver: "Tx", Method: "ExecContext", Pointer: true, CheckArgs: []int{2}},
			{Package: "database/sql", Receiver: "Tx", Method: "Prepare", Pointer: true, CheckArgs: []int{1}},
			{Package: "database/sql", Receiver: "Tx", Method: "PrepareContext", Pointer: true, CheckArgs: []int{2}},
		},
		Sanitizers: []taint.Sanitizer{
			// No stdlib sanitizers for SQL — use parameterized queries instead.
			// The CheckArgs configuration already excludes prepared statement params.
		},
	}
}

// newSQLInjectionAnalyzer creates an analyzer for detecting SQL injection vulnerabilities
// via taint analysis (G701)
func newSQLInjectionAnalyzer(id string, description string) *analysis.Analyzer {
	config := SQLInjection()
	rule := SQLInjectionRule
	rule.ID = id
	rule.Description = description
	return taint.NewGosecAnalyzer(&rule, &config)
}
