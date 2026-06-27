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

// PathTraversal returns a configuration for detecting path traversal vulnerabilities.
func PathTraversal() taint.Config {
	return taint.Config{
		Sources: []taint.Source{
			// Type sources: tainted when received as function parameters
			{Package: "net/http", Name: "Request", Pointer: true},
			{Package: "net/url", Name: "URL", Pointer: true},
			{Package: "bufio", Name: "Reader", Pointer: true},
			{Package: "bufio", Name: "Scanner", Pointer: true},

			// Function sources: always produce tainted data
			{Package: "os", Name: "Args", IsFunc: true},
			{Package: "os", Name: "Getenv", IsFunc: true},
			{Package: "os", Name: "ReadFile", IsFunc: true},

			// NOTE: os.File is NOT a source type. Reading from a locally-opened file
			// with a hardcoded path is not tainted. The file path argument to os.Open
			// is what the sink checks. If someone opens a file from user input and
			// then reads from it, the taint flows through the path argument, not the
			// File type itself.
		},
		Sinks: []taint.Sink{
			{Package: "os", Method: "Open"},
			{Package: "os", Method: "OpenFile"},
			{Package: "os", Method: "Create"},
			{Package: "os", Method: "ReadFile"},
			{Package: "os", Method: "WriteFile"},
			{Package: "os", Method: "Remove"},
			{Package: "os", Method: "RemoveAll"},
			{Package: "os", Method: "Rename"},
			{Package: "os", Method: "Mkdir"},
			{Package: "os", Method: "MkdirAll"},
			{Package: "os", Method: "Stat"},
			{Package: "os", Method: "Lstat"},
			{Package: "os", Method: "Chmod"},
			{Package: "os", Method: "Chown"},
			{Package: "io/ioutil", Method: "ReadFile"},
			{Package: "io/ioutil", Method: "WriteFile"},
			{Package: "io/ioutil", Method: "ReadDir"},
			{Package: "path/filepath", Method: "Walk"},
			{Package: "path/filepath", Method: "WalkDir"},
			// HTTP file-serving functions: user-controlled path = arbitrary file read
			{Package: "net/http", Method: "ServeFile", CheckArgs: []int{2}},
			{Package: "net/http", Method: "ServeFileFS", CheckArgs: []int{3}},
		},
		Sanitizers: []taint.Sanitizer{
			// filepath.Clean normalizes and removes traversal components
			{Package: "path/filepath", Method: "Clean"},
			// filepath.Abs calls Clean internally (per Go docs)
			{Package: "path/filepath", Method: "Abs"},
			// filepath.Base extracts just the filename, removing directory traversal
			{Package: "path/filepath", Method: "Base"},
			// filepath.Rel computes a relative path safely
			{Package: "path/filepath", Method: "Rel"},
			// url.PathEscape escapes path components
			{Package: "net/url", Method: "PathEscape"},

			// path.Base and path.Clean provide identical traversal-stripping
			// semantics as their filepath counterparts (the only difference is
			// separator handling, which is irrelevant for security).
			{Package: "path", Method: "Base"},
			{Package: "path", Method: "Clean"},

			// Integer conversions eliminate path traversal vectors entirely —
			// the result can never contain "/" or ".." characters.
			{Package: "strconv", Method: "Atoi"},
			{Package: "strconv", Method: "ParseInt"},
			{Package: "strconv", Method: "ParseUint"},
			{Package: "strconv", Method: "ParseFloat"},
			{Package: "strconv", Method: "ParseBool"},
		},
	}
}

// newPathTraversalAnalyzer creates an analyzer for detecting path traversal vulnerabilities
// via taint analysis (G703)
func newPathTraversalAnalyzer(id string, description string) *analysis.Analyzer {
	config := PathTraversal()
	rule := PathTraversalRule
	rule.ID = id
	rule.Description = description
	return taint.NewGosecAnalyzer(&rule, &config)
}
