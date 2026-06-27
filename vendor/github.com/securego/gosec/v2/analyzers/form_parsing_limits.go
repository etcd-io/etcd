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

// FormParsingLimits returns a taint analysis configuration for detecting
// unbounded multipart form parsing in HTTP handlers.
//
// Only ParseMultipartForm is flagged because ParseForm, FormValue, and
// PostFormValue already enforce a built-in 10 MiB body limit in Go's
// standard library (see net/http.Request.ParseForm documentation).
func FormParsingLimits() taint.Config {
	return taint.Config{
		Sources: []taint.Source{
			{Package: "net/http", Name: "Request", Pointer: true},
		},
		Sinks: []taint.Sink{
			// ParseMultipartForm reads the entire body into memory/disk with
			// no automatic cap — the caller-supplied maxMemory only limits the
			// in-memory portion while the total can be maxMemory + 10 MiB.
			// Without http.MaxBytesReader the full body is consumed.
			// CheckArgs: [0] checks only the receiver (*http.Request).
			{Package: "net/http", Receiver: "Request", Method: "ParseMultipartForm", Pointer: true, CheckArgs: []int{0}},
		},
		Sanitizers: []taint.Sanitizer{},
	}
}

func newFormParsingLimitAnalyzer(id string, description string) *analysis.Analyzer {
	config := FormParsingLimits()
	rule := FormParsingLimitRule
	rule.ID = id
	rule.Description = description
	return taint.NewGosecAnalyzer(&rule, &config)
}
