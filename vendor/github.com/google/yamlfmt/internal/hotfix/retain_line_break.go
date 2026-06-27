// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The features in this file are to retain line breaks.
// The basic idea is to insert/remove placeholder comments in the yaml document before and after the format process.

package hotfix

import (
	"bufio"
	"bytes"
	"context"
	"strings"

	"github.com/google/yamlfmt"
)

const lineBreakPlaceholder = "#magic___^_^___line"

type paddinger struct {
	strings.Builder
}

func (p *paddinger) adjust(txt string) {
	var indentSize int
	for i := 0; i < len(txt) && txt[i] == ' '; i++ { // yaml only allows space to indent.
		indentSize++
	}
	// Grows if the given size is larger than us and always return the max padding.
	for diff := indentSize - p.Len(); diff > 0; diff-- {
		p.WriteByte(' ')
	}
}

func MakeFeatureRetainLineBreak(linebreakStr string, chomp bool) yamlfmt.Feature {
	return yamlfmt.Feature{
		Name:         "Retain Line Breaks",
		BeforeAction: replaceLineBreakFeature(linebreakStr, chomp),
		AfterAction:  restoreLineBreakFeature(linebreakStr),
	}
}

func replaceLineBreakFeature(newlineStr string, chomp bool) yamlfmt.FeatureFunc {
	return func(_ context.Context, content []byte) (context.Context, []byte, error) {
		var buf bytes.Buffer
		reader := bytes.NewReader(content)
		scanner := bufio.NewScanner(reader)
		var inLineBreaks bool
		var padding paddinger
		for scanner.Scan() {
			txt := scanner.Text()
			padding.adjust(txt)
			if strings.TrimSpace(txt) == "" { // line break or empty space line.
				if chomp && inLineBreaks {
					continue
				}
				buf.WriteString(padding.String()) // prepend some padding incase literal multiline strings.
				buf.WriteString(lineBreakPlaceholder)
				buf.WriteString(newlineStr)
				inLineBreaks = true
			} else {
				buf.WriteString(txt)
				buf.WriteString(newlineStr)
				inLineBreaks = false
			}
		}
		return nil, buf.Bytes(), scanner.Err()
	}
}

func restoreLineBreakFeature(newlineStr string) yamlfmt.FeatureFunc {
	return func(_ context.Context, content []byte) (context.Context, []byte, error) {
		var buf bytes.Buffer
		reader := bytes.NewReader(content)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			txt := scanner.Text()
			if strings.TrimSpace(txt) == "" {
				// The basic yaml lib inserts newline when there is a comment(either placeholder or by user)
				// followed by optional line breaks and a `---` multi-documents.
				// To fix it, the empty line could only be inserted by us.
				continue
			}
			if strings.HasPrefix(strings.TrimLeft(txt, " "), lineBreakPlaceholder) {
				buf.WriteString(newlineStr)
				continue
			}
			buf.WriteString(txt)
			buf.WriteString(newlineStr)
		}
		return nil, buf.Bytes(), scanner.Err()
	}
}
