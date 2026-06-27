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

// SMTPInjection returns a configuration for detecting SMTP command/header injection vulnerabilities.
func SMTPInjection() taint.Config {
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
			// net/smtp.SendMail(addr, auth, from, to, msg)
			// Check sender and recipient envelope fields.
			{Package: "net/smtp", Method: "SendMail", CheckArgs: []int{2, 3}},

			// For smtp.Client methods, Args[0] is receiver.
			{Package: "net/smtp", Receiver: "Client", Method: "Mail", Pointer: true, CheckArgs: []int{1}},
			{Package: "net/smtp", Receiver: "Client", Method: "Rcpt", Pointer: true, CheckArgs: []int{1}},
		},
		Sanitizers: []taint.Sanitizer{
			// net/mail parsers enforce RFC-compatible mailbox/address syntax.
			{Package: "net/mail", Method: "ParseAddress"},
			{Package: "net/mail", Method: "ParseAddressList"},

			// AddressParser methods also provide structured parsing.
			{Package: "net/mail", Receiver: "AddressParser", Method: "Parse", Pointer: true},
			{Package: "net/mail", Receiver: "AddressParser", Method: "ParseList", Pointer: true},
		},
	}
}

// newSMTPInjectionAnalyzer creates an analyzer for detecting SMTP injection vulnerabilities
// via taint analysis (G707)
func newSMTPInjectionAnalyzer(id string, description string) *analysis.Analyzer {
	config := SMTPInjection()
	rule := SMTPInjectionRule
	rule.ID = id
	rule.Description = description
	return taint.NewGosecAnalyzer(&rule, &config)
}
