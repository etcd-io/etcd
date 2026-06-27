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

// SSRF returns a configuration for detecting Server-Side Request Forgery vulnerabilities.
func SSRF() taint.Config {
	return taint.Config{
		Sources: []taint.Source{
			// Type sources: tainted when received as function parameters from external callers
			{Package: "net/http", Name: "Request", Pointer: true},

			// Function sources: always produce tainted data
			{Package: "os", Name: "Args", IsFunc: true},
			{Package: "os", Name: "Getenv", IsFunc: true},

			// I/O sources that read from external input
			{Package: "bufio", Name: "Reader", Pointer: true},
			{Package: "bufio", Name: "Scanner", Pointer: true},

			// NOTE: *os.File is NOT a source type here. A file opened with a
			// hardcoded path (e.g., config file) is not an external input source.
			// If the file was opened from user-controlled input, the taint would
			// flow through the path argument, and that's a path traversal issue (G703),
			// not SSRF.
		},
		Sinks: []taint.Sink{
			// URL argument is what we check - these are the first data arg
			{Package: "net/http", Method: "Get", CheckArgs: []int{0}},
			{Package: "net/http", Method: "Post", CheckArgs: []int{0}},
			{Package: "net/http", Method: "Head", CheckArgs: []int{0}},
			{Package: "net/http", Method: "PostForm", CheckArgs: []int{0}},
			// NewRequest/NewRequestWithContext: URL is arg index 1 (method=0, url=1, body=2)
			// or for WithContext: ctx=0, method=1, url=2, body=3
			{Package: "net/http", Method: "NewRequest", CheckArgs: []int{1}},
			{Package: "net/http", Method: "NewRequestWithContext", CheckArgs: []int{2}},
			// Client methods - the request object carries the taint
			{Package: "net/http", Receiver: "Client", Method: "Do", Pointer: true, CheckArgs: []int{1}},
			{Package: "net/http", Receiver: "Client", Method: "Get", Pointer: true, CheckArgs: []int{1}},
			{Package: "net/http", Receiver: "Client", Method: "Post", Pointer: true, CheckArgs: []int{1}},
			{Package: "net/http", Receiver: "Client", Method: "Head", Pointer: true, CheckArgs: []int{1}},
			{Package: "net", Method: "Dial", CheckArgs: []int{1}},
			{Package: "net", Method: "DialTimeout", CheckArgs: []int{1}},
			{Package: "net", Method: "LookupHost", CheckArgs: []int{0}},
			{Package: "net/http/httputil", Method: "NewSingleHostReverseProxy", CheckArgs: []int{0}},
		},
		Sanitizers: []taint.Sanitizer{
			// URL validation/parsing that enforces allowlists would be custom;
			// there are no stdlib sanitizers that truly prevent SSRF.
			// However, url.Parse itself is not a sanitizer — it doesn't restrict
			// which hosts can be accessed.
		},
	}
}

// newSSRFAnalyzer creates an analyzer for detecting SSRF vulnerabilities
// via taint analysis (G704)
func newSSRFAnalyzer(id string, description string) *analysis.Analyzer {
	config := SSRF()
	rule := SSRFRule
	rule.ID = id
	rule.Description = description
	return taint.NewGosecAnalyzer(&rule, &config)
}
