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

// CommandInjection returns a configuration for detecting command injection vulnerabilities.
func CommandInjection() taint.Config {
	return taint.Config{
		Sources: []taint.Source{
			// Type sources: tainted when received as parameters
			{Package: "net/http", Name: "Request", Pointer: true},
			{Package: "bufio", Name: "Reader", Pointer: true},
			{Package: "bufio", Name: "Scanner", Pointer: true},

			// Function sources
			{Package: "os", Name: "Args", IsFunc: true},
			{Package: "os", Name: "Getenv", IsFunc: true},
		},
		Sinks: []taint.Sink{
			// Detect at command creation, not execution (avoids double detection)
			{Package: "os/exec", Method: "Command"},
			{Package: "os/exec", Method: "CommandContext"},
			{Package: "os", Method: "StartProcess"},
			{Package: "syscall", Method: "Exec"},
			{Package: "syscall", Method: "ForkExec"},
			{Package: "syscall", Method: "StartProcess"},
		},
		Sanitizers: []taint.Sanitizer{
			// No general-purpose stdlib sanitizer for command injection.
			// The proper fix is to use exec.Command with separate args, not shell strings.
		},
	}
}

// newCommandInjectionAnalyzer creates an analyzer for detecting command injection vulnerabilities
// via taint analysis (G702)
func newCommandInjectionAnalyzer(id string, description string) *analysis.Analyzer {
	config := CommandInjection()
	rule := CommandInjectionRule
	rule.ID = id
	rule.Description = description
	return taint.NewGosecAnalyzer(&rule, &config)
}
