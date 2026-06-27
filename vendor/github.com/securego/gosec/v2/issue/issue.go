// (c) Copyright 2016 Hewlett Packard Enterprise Development LP
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

package issue

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"strconv"

	"github.com/securego/gosec/v2/cwe"
)

// Score type used by severity and confidence values
type Score int

const (
	// Low severity or confidence
	Low Score = iota
	// Medium severity or confidence
	Medium
	// High severity or confidence
	High
)

// SnippetOffset defines the number of lines captured before
// the beginning and after the end of a code snippet
const SnippetOffset = 1

// GetCweByRule retrieves a cwe weakness for a given RuleID
func GetCweByRule(id string) *cwe.Weakness {
	cweID, ok := ruleToCWE[id]
	if ok && cweID != "" {
		return cwe.Get(cweID)
	}
	return nil
}

// ruleToCWE maps gosec rules to CWEs. The key is the rule ID
// and the value is the CWE ID. If a rule does not have a CWE,
// it will not be included in this map.
var ruleToCWE = map[string]string{
	"G101": "798",
	"G102": "200",
	"G103": "242",
	"G104": "703",
	"G106": "322",
	"G107": "88",
	"G108": "200",
	"G109": "190",
	"G110": "409",
	"G111": "22",
	"G112": "400",
	"G113": "444",
	"G707": "93",
	"G708": "94",
	"G709": "502",
	"G114": "676",
	"G115": "190",
	"G116": "838",
	"G117": "499",
	"G118": "400",
	"G119": "200",
	"G120": "400",
	"G121": "346",
	"G122": "367",
	"G123": "295",
	"G124": "614",
	"G201": "89",
	"G202": "89",
	"G203": "79",
	"G204": "78",
	"G301": "276",
	"G302": "276",
	"G303": "377",
	"G304": "22",
	"G305": "22",
	"G306": "276",
	"G307": "276",
	"G401": "328",
	"G402": "295",
	"G403": "310",
	"G404": "338",
	"G405": "327",
	"G406": "328",
	"G407": "1204",
	"G408": "287",
	"G501": "327",
	"G502": "327",
	"G503": "327",
	"G504": "327",
	"G505": "327",
	"G506": "327",
	"G507": "327",
	"G601": "118",
	"G602": "118",
	"G701": "89",
	"G702": "78",
	"G703": "22",
	"G704": "918",
	"G705": "79",
	"G706": "117",
	"G710": "601",
}

// Issue is returned by a gosec rule if it discovers an issue with the scanned code.
type Issue struct {
	Severity     Score             `json:"severity"`          // issue severity (how problematic it is)
	Confidence   Score             `json:"confidence"`        // issue confidence (how sure we are we found it)
	Cwe          *cwe.Weakness     `json:"cwe"`               // Cwe associated with RuleID
	RuleID       string            `json:"rule_id"`           // Human readable explanation
	What         string            `json:"details"`           // Human readable explanation
	File         string            `json:"file"`              // File name we found it in
	Code         string            `json:"code"`              // Impacted code line
	Line         string            `json:"line"`              // Line number in file
	Col          string            `json:"column"`            // Column number in line
	NoSec        bool              `json:"nosec"`             // true if the issue is nosec
	Suppressions []SuppressionInfo `json:"suppressions"`      // Suppression info of the issue
	Autofix      string            `json:"autofix,omitempty"` // Proposed auto fix the issue
}

// SuppressionInfo object is to record the kind and the justification that used
// to suppress violations.
type SuppressionInfo struct {
	Kind          string `json:"kind"`
	Justification string `json:"justification"`
}

// FileLocation point out the file path and line number in file
func (i *Issue) FileLocation() string {
	return fmt.Sprintf("%s:%s", i.File, i.Line)
}

// MetaData is embedded in all gosec rules. The Severity, Confidence and What message
// will be passed through to reported issues.
type MetaData struct {
	RuleID     string
	Severity   Score
	Confidence Score
	What       string
}

// NewMetaData creates a new MetaData object
func NewMetaData(id, what string, severity, confidence Score) MetaData {
	return MetaData{
		RuleID:     id,
		What:       what,
		Severity:   severity,
		Confidence: confidence,
	}
}

// ID returns the rule ID. This satisfies part of the gosec.Rule interface
// when MetaData is embedded in a rule struct.
func (m MetaData) ID() string {
	return m.RuleID
}

// MarshalJSON is used convert a Score object into a JSON representation
func (c Score) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// String converts a Score into a string
func (c Score) String() string {
	switch c {
	case High:
		return "HIGH"
	case Medium:
		return "MEDIUM"
	case Low:
		return "LOW"
	}
	return "UNDEFINED"
}

// CodeSnippet extracts a code snippet based on the ast reference
func CodeSnippet(file *os.File, start int64, end int64) (string, error) {
	var pos int64
	var buf bytes.Buffer
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		pos++
		if pos > end {
			break
		} else if pos >= start && pos <= end {
			code := fmt.Sprintf("%d: %s\n", pos, scanner.Text())
			buf.WriteString(code)
		}
	}
	return buf.String(), nil
}

func codeSnippetStartLine(node ast.Node, fobj *token.File) int64 {
	s := (int64)(fobj.Line(node.Pos()))
	if s-SnippetOffset > 0 {
		return s - SnippetOffset
	}
	return s
}

func codeSnippetEndLine(node ast.Node, fobj *token.File) int64 {
	e := (int64)(fobj.Line(node.End()))
	return e + SnippetOffset
}

// New creates a new Issue
func New(fobj *token.File, node ast.Node, ruleID, desc string, severity, confidence Score) *Issue {
	name := fobj.Name()
	var line string
	var col string

	if node == nil {
		line = "0"
		col = "0"
	} else {
		line = GetLine(fobj, node)
		col = strconv.Itoa(fobj.Position(node.Pos()).Column)
	}

	var code string
	if node == nil {
		code = "invalid AST node provided"
	}
	if file, err := os.Open(fobj.Name()); err == nil && node != nil {
		defer file.Close() // #nosec
		s := codeSnippetStartLine(node, fobj)
		e := codeSnippetEndLine(node, fobj)
		code, err = CodeSnippet(file, s, e)
		if err != nil {
			code = err.Error()
		}
	}

	return &Issue{
		File:       name,
		Line:       line,
		Col:        col,
		RuleID:     ruleID,
		What:       desc,
		Confidence: confidence,
		Severity:   severity,
		Code:       code,
		Cwe:        GetCweByRule(ruleID),
	}
}

// WithSuppressions set the suppressions of the issue
func (i *Issue) WithSuppressions(suppressions []SuppressionInfo) *Issue {
	i.Suppressions = suppressions
	return i
}

// GetLine returns the line number of a given ast.Node
func GetLine(fobj *token.File, node ast.Node) string {
	start, end := fobj.Line(node.Pos()), fobj.Line(node.End())
	line := strconv.Itoa(start)
	if start != end {
		line = fmt.Sprintf("%d-%d", start, end)
	}
	return line
}
