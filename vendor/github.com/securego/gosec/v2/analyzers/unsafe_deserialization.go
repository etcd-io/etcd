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

// UnsafeDeserialization returns a configuration for detecting unsafe
// deserialization of untrusted data.
//
// Go's encoding/gob package embeds full type information in its wire format
// and will instantiate arbitrary registered types during decode. When an HTTP
// handler passes r.Body directly to gob.NewDecoder().Decode(), an attacker
// controls which types get instantiated, leading to denial-of-service or
// potential RCE (CVE-2024-34156).
//
// gopkg.in/yaml.v2's Unmarshal into interface{} can instantiate arbitrary Go
// types via YAML tags. encoding/xml is susceptible to deeply-nested structure
// DoS and external entity expansion.
func UnsafeDeserialization() taint.Config {
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
			// encoding/gob — highest risk: arbitrary type instantiation from wire format
			// gob.NewDecoder takes an io.Reader (arg 0), so if the reader is tainted
			// the decoder will process attacker-controlled data.
			{Package: "encoding/gob", Method: "NewDecoder", CheckArgs: []int{0}},

			// gopkg.in/yaml.v2 — Unmarshal([]byte, interface{}) can instantiate arbitrary types
			{Package: "gopkg.in/yaml.v2", Method: "Unmarshal", CheckArgs: []int{0}},
			// yaml.NewDecoder takes an io.Reader (arg 0)
			{Package: "gopkg.in/yaml.v2", Method: "NewDecoder", CheckArgs: []int{0}},

			// encoding/xml — deeply-nested structure DoS, entity expansion
			{Package: "encoding/xml", Method: "NewDecoder", CheckArgs: []int{0}},
			{Package: "encoding/xml", Method: "Unmarshal", CheckArgs: []int{0}},
		},
		Sanitizers: []taint.Sanitizer{
			// io.LimitReader bounds the amount of data read, mitigating DoS amplification
			{Package: "io", Method: "LimitReader"},

			// Numeric conversions — result is safe
			{Package: "strconv", Method: "Atoi"},
			{Package: "strconv", Method: "Itoa"},
			{Package: "strconv", Method: "ParseInt"},
			{Package: "strconv", Method: "ParseUint"},
			{Package: "strconv", Method: "ParseFloat"},
		},
	}
}

// newUnsafeDeserializationAnalyzer creates an analyzer for detecting unsafe
// deserialization of untrusted data via taint analysis (G709).
func newUnsafeDeserializationAnalyzer(id string, description string) *analysis.Analyzer {
	config := UnsafeDeserialization()
	rule := UnsafeDeserializationRule
	rule.ID = id
	rule.Description = description
	return taint.NewGosecAnalyzer(&rule, &config)
}
