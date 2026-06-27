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

// OpenRedirect returns a configuration for detecting open-redirect vulnerabilities
// where user-controlled data flows into the URL argument of net/http.Redirect.
// See CWE-601.
func OpenRedirect() taint.Config {
	return taint.Config{
		Sources: []taint.Source{
			// Type sources: tainted when received as parameters from external callers.
			// Any read from a *http.Request (FormValue, URL.Query().Get, Cookie, etc.)
			// propagates taint through the existing receiver-based logic in isTainted.
			{Package: "net/http", Name: "Request", Pointer: true},
			{Package: "net/url", Name: "URL", Pointer: true},
			{Package: "net/url", Name: "Values"},
		},
		Sinks: []taint.Sink{
			// http.Redirect(w, r, url, code): only the URL string (arg index 2)
			// is the redirect target. Skipping arg 1 (*http.Request) prevents the
			// receiver itself from being treated as a tainted sink argument.
			{Package: "net/http", Method: "Redirect", CheckArgs: []int{2}},
		},
		Sanitizers: []taint.Sanitizer{
			// url.PathEscape / QueryEscape neutralize untrusted path or query
			// fragments embedded into a hard-coded base URL.
			{Package: "net/url", Method: "PathEscape"},
			{Package: "net/url", Method: "QueryEscape"},

			// Numeric conversions cannot produce a URL host or scheme.
			{Package: "strconv", Method: "Atoi"},
			{Package: "strconv", Method: "Itoa"},
			{Package: "strconv", Method: "ParseInt"},
			{Package: "strconv", Method: "ParseUint"},
			{Package: "strconv", Method: "FormatInt"},
			{Package: "strconv", Method: "FormatUint"},
		},
	}
}

// newOpenRedirectAnalyzer creates an analyzer for detecting open-redirect
// vulnerabilities via taint analysis (G710).
func newOpenRedirectAnalyzer(id string, description string) *analysis.Analyzer {
	config := OpenRedirect()
	rule := OpenRedirectRule
	rule.ID = id
	rule.Description = description
	return taint.NewGosecAnalyzer(&rule, &config)
}
