// Copyright (c) 2020-2024 Denis Tingaikin
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package goheader

import (
	"fmt"
	"go/ast"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Target struct {
	Path string
	File *ast.File
}

const iso = "2006-01-02 15:04:05 -0700"

func (t *Target) ModTime() (time.Time, error) {
	diff, err := exec.Command("git", "diff", t.Path).CombinedOutput()
	if err == nil && len(diff) == 0 {
		line, err := exec.Command("git", "log", "-1", "--pretty=format:%cd", "--date=iso", "--", t.Path).CombinedOutput()
		if err == nil {
			return time.Parse(iso, string(line))
		}
	}
	info, err := os.Stat(t.Path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

type Analyzer struct {
	values   map[string]Value
	template string
}

func (a *Analyzer) processPerTargetValues(target *Target) error {
	a.values["mod-year"] = a.values["year"]
	a.values["mod-year-range"] = a.values["year-range"]
	if t, err := target.ModTime(); err == nil {
		a.values["mod-year"] = &ConstValue{RawValue: fmt.Sprint(t.Year())}
		a.values["mod-year-range"] = &RegexpValue{RawValue: `((20\d\d\-{{mod-year}})|({{mod-year}}))`}
	}

	for _, v := range a.values {
		if err := v.Calculate(a.values); err != nil {
			return err
		}
	}
	return nil
}

func (a *Analyzer) Analyze(target *Target) (i Issue) {
	if a.template == "" {
		return NewIssue("Missed template for check")
	}

	if err := a.processPerTargetValues(target); err != nil {
		return &issue{msg: err.Error()}
	}

	file := target.File
	var header string
	var offset = Location{
		Position: 1,
	}
	if len(file.Comments) > 0 && file.Comments[0].Pos() < file.Package {
		if strings.HasPrefix(file.Comments[0].List[0].Text, "/*") {
			header = (&ast.CommentGroup{List: []*ast.Comment{file.Comments[0].List[0]}}).Text()
		} else {
			header = file.Comments[0].Text()
			offset.Position += 3
		}
	}
	defer func() {
		if i == nil {
			return
		}
		fix, ok := a.generateFix(i, file, header)
		if !ok {
			return
		}
		i = NewIssueWithFix(i.Message(), i.Location(), fix)
	}()
	header = strings.TrimSpace(header)
	if header == "" {
		return NewIssue("Missed header for check")
	}
	s := NewReader(header)
	s.SetOffset(offset)
	t := NewReader(a.template)
	for !s.Done() && !t.Done() {
		templateCh := t.Peek()
		if templateCh == '{' {
			name := a.readField(t)
			if a.values[name] == nil {
				return NewIssue(fmt.Sprintf("Template has unknown value: %v", name))
			}
			if i := a.values[name].Read(s); i != nil {
				return i
			}
			continue
		}
		sourceCh := s.Peek()
		if sourceCh != templateCh {
			l := s.Location()
			notNextLine := func(r rune) bool {
				return r != '\n'
			}
			actual := s.ReadWhile(notNextLine)
			expected := t.ReadWhile(notNextLine)
			return NewIssueWithLocation(fmt.Sprintf("Actual: %v\nExpected:%v", actual, expected), l)
		}
		s.Next()
		t.Next()
	}
	if !s.Done() {
		l := s.Location()
		return NewIssueWithLocation(fmt.Sprintf("Unexpected string: %v", s.Finish()), l)
	}
	if !t.Done() {
		l := s.Location()
		return NewIssueWithLocation(fmt.Sprintf("Missed string: %v", t.Finish()), l)
	}
	return nil
}

func (a *Analyzer) readField(reader *Reader) string {
	_ = reader.Next()
	_ = reader.Next()

	r := reader.ReadWhile(func(r rune) bool {
		return r != '}'
	})

	_ = reader.Next()
	_ = reader.Next()

	return strings.ToLower(strings.TrimSpace(r))
}

func New(options ...Option) *Analyzer {
	a := &Analyzer{values: make(map[string]Value)}
	for _, o := range options {
		o.apply(a)
	}
	return a
}

func (a *Analyzer) generateFix(i Issue, file *ast.File, header string) (Fix, bool) {
	var expect string
	t := NewReader(a.template)
	for !t.Done() {
		ch := t.Peek()
		if ch == '{' {
			f := a.values[a.readField(t)]
			if f == nil {
				return Fix{}, false
			}
			if f.Calculate(a.values) != nil {
				return Fix{}, false
			}
			expect += f.Get()
			continue
		}

		expect += string(ch)
		t.Next()
	}

	fix := Fix{Expected: strings.Split(expect, "\n")}
	if !(len(file.Comments) > 0 && file.Comments[0].Pos() < file.Package) {
		for i := range fix.Expected {
			fix.Expected[i] = "// " + fix.Expected[i]
		}
		return fix, true
	}

	actual := file.Comments[0].List[0].Text
	if !strings.HasPrefix(actual, "/*") {
		for i := range fix.Expected {
			fix.Expected[i] = "// " + fix.Expected[i]
		}
		for _, c := range file.Comments[0].List {
			fix.Actual = append(fix.Actual, c.Text)
		}
		i = NewIssueWithFix(i.Message(), i.Location(), fix)
		return fix, true
	}

	gets := func(i int, end bool) string {
		if i < 0 {
			return header
		}
		if end {
			return header[i+1:]
		}
		return header[:i]
	}
	start := strings.Index(actual, gets(strings.IndexByte(header, '\n'), false))
	if start < 0 {
		return Fix{}, false // Should be impossible
	}
	nl := strings.LastIndexByte(actual[:start], '\n')
	if nl >= 0 {
		fix.Actual = strings.Split(actual[:nl], "\n")
		fix.Expected = append(fix.Actual, fix.Expected...)
		actual = actual[nl+1:]
		start -= nl + 1
	}

	prefix := actual[:start]
	if nl < 0 {
		fix.Expected[0] = prefix + fix.Expected[0]
	} else {
		n := len(fix.Actual)
		for i := range fix.Expected[n:] {
			fix.Expected[n+i] = prefix + fix.Expected[n+i]
		}
	}

	last := gets(strings.LastIndexByte(header, '\n'), true)
	end := strings.Index(actual, last)
	if end < 0 {
		return Fix{}, false // Should be impossible
	}

	trailing := actual[end+len(last):]
	if i := strings.IndexRune(trailing, '\n'); i < 0 {
		fix.Expected[len(fix.Expected)-1] += trailing
	} else {
		fix.Expected[len(fix.Expected)-1] += trailing[:i]
		fix.Expected = append(fix.Expected, strings.Split(trailing[i+1:], "\n")...)
	}

	fix.Actual = append(fix.Actual, strings.Split(actual, "\n")...)
	return fix, true
}
