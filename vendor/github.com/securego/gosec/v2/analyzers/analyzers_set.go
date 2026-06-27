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

import "golang.org/x/tools/go/analysis"

type AnalyzerSet struct {
	Analyzers             []*analysis.Analyzer
	AnalyzerSuppressedMap map[string]bool
}

// NewAnalyzerSet constructs a new AnalyzerSet
func NewAnalyzerSet() *AnalyzerSet {
	return &AnalyzerSet{nil, make(map[string]bool)}
}

// Register adds a trigger for the supplied analyzer
func (a *AnalyzerSet) Register(analyzer *analysis.Analyzer, isSuppressed bool) {
	a.Analyzers = append(a.Analyzers, analyzer)
	a.AnalyzerSuppressedMap[analyzer.Name] = isSuppressed
}

// IsSuppressed will return whether the Analyzer is suppressed.
func (a *AnalyzerSet) IsSuppressed(ruleID string) bool {
	return a.AnalyzerSuppressedMap[ruleID]
}
